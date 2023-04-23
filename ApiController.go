package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/berejant/go-kneu"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/kneu-messenger-pigeon/events"
	"github.com/segmentio/kafka-go"
	"html"
	"io"
	"net/http"
	"strconv"
	"time"
)

const adminUserid = 1

type ApiController struct {
	out              io.Writer
	config           Config
	writer           events.WriterInterface
	oauthClient      kneu.OauthClientInterface
	apiClientFactory func(token string) kneu.ApiClientInterface

	oauthRedirectUrl string
}

type AuthOptionsClaims struct {
	jwt.RegisteredClaims
	Client       string `form:"client" json:"client" binding:"required"`
	ClientUserId string `form:"client_user_id" json:"clientUserId"  binding:"required"`
	RedirectUri  string `form:"redirect_uri" json:"redirectUri,omitempty"`
	KneuUserId   uint   `form:"-" json:"userId,omitempty"`
}

type Student struct {
	Id         uint
	LastName   string
	FirstName  string
	MiddleName string
	Gender     events.Gender
}

func (controller *ApiController) setupRouter() *gin.Engine {
	router := gin.New()

	completeUri := "/complete"
	controller.oauthRedirectUrl = controller.config.publicUrl + completeUri

	router.LoadHTMLGlob("templates/*")
	router.POST("/url", gin.BasicAuth(gin.Accounts{
		"pigeon": controller.config.appSecret,
	}), controller.getAuthUrl)

	router.GET(completeUri, controller.completeAuth)
	router.POST("/admin", controller.completeAdminAuth)

	router.StaticFile("/close.html", "./templates/close.html")

	router.GET("/healthcheck", func(c *gin.Context) {
		c.String(http.StatusOK, "health")
	})

	return router
}

func (controller *ApiController) getAuthUrl(c *gin.Context) {
	var err error
	var state string

	authOptionsClaims := AuthOptionsClaims{}
	err = c.Bind(&authOptionsClaims)
	if err == nil {
		authOptionsClaims.KneuUserId = 0
		authOptionsClaims.Issuer = "pigeonAuthorizer"
		authOptionsClaims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Minute * 15))

		state, err = controller.buildState(authOptionsClaims)
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Wrong request data"})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"authUrl": controller.oauthClient.GetOauthUrl(controller.oauthRedirectUrl, state),
		})
	}
}

func (controller *ApiController) completeAuth(c *gin.Context) {
	var authOptionsClaims AuthOptionsClaims
	var tokenResponse kneu.OauthTokenResponse
	var userMeResponse kneu.UserMeResponse

	code := c.Query("code")
	state := c.Query("state")

	authOptionsClaims, err := controller.parseState(state)
	if err == nil {
		tokenResponse, err = controller.oauthClient.GetOauthToken(controller.oauthRedirectUrl, code)
	}

	if err == nil && tokenResponse.UserId == adminUserid {
		authOptionsClaims.KneuUserId = tokenResponse.UserId

		state, err = controller.buildState(authOptionsClaims)
		if err == nil {
			controller.responseWithAdminAuthFrom(c, state)
			return
		}
	}

	if err == nil {
		userMeResponse, err = controller.apiClientFactory(tokenResponse.AccessToken).GetUserMe()
	}

	if err == nil && userMeResponse.Type != "student" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": html.EscapeString("Вам потрібно використати особистий кабінет студента"),
		})
		return
	}

	if err == nil {
		err = controller.finishAuthorization(authOptionsClaims, Student{
			Id:         userMeResponse.StudentId,
			LastName:   userMeResponse.LastName,
			FirstName:  userMeResponse.FirstName,
			MiddleName: userMeResponse.MiddleName,
			Gender:     events.GenderFromString(userMeResponse.Gender),
		})
	}

	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{})
		return
	} else {
		controller.successRedirect(c, authOptionsClaims)
	}
}

func (controller *ApiController) successRedirect(c *gin.Context, claims AuthOptionsClaims) {
	redirectUri := claims.RedirectUri
	if redirectUri == "" {
		redirectUri = controller.config.publicUrl + "/close.html"
	}

	c.Redirect(http.StatusFound, redirectUri)
}

func (controller *ApiController) responseWithAdminAuthFrom(c *gin.Context, state string) {
	c.Header("Content-Security-Policy", "base-uri 'none'; object-src 'none'; script-src 'none';")

	c.HTML(http.StatusOK, "auth_admin.html", gin.H{
		"state":     html.EscapeString(state),
		"actionUrl": "/admin",
	})
}

func (controller *ApiController) completeAdminAuth(c *gin.Context) {
	var studentId uint64
	c.Header("Content-Security-Policy", "base-uri 'none'; object-src 'none'; script-src 'none';")

	authOptionsClaims, err := controller.parseState(c.PostForm("state"))
	if err == nil {
		studentId, err = strconv.ParseUint(c.PostForm("student_id"), 10, 0)
	}

	if err == nil {
		if studentId <= 0 {
			controller.responseWithAdminAuthFrom(c, c.PostForm("state"))
			return
		}

		err = errors.New("not enough rights")
		if adminUserid == authOptionsClaims.KneuUserId {
			err = controller.finishAuthorization(authOptionsClaims, Student{
				Id:         uint(studentId),
				LastName:   "Адмін",
				FirstName:  "Адмін",
				MiddleName: "Адмін",
				Gender:     events.UnknownGender,
			})
			if err == nil {
				controller.successRedirect(c, authOptionsClaims)
				return
			}
		}
	}

	c.HTML(http.StatusBadRequest, "error.html", gin.H{
		"error": html.EscapeString(err.Error()),
	})
}

func (controller *ApiController) finishAuthorization(claims AuthOptionsClaims, student Student) error {
	event := events.UserAuthorizedEvent{
		Client:       claims.Client,
		ClientUserId: claims.ClientUserId,
		StudentId:    student.Id,
		LastName:     student.LastName,
		FirstName:    student.FirstName,
		MiddleName:   student.MiddleName,
		Gender:       student.Gender,
	}

	payload, _ := json.Marshal(event)
	return controller.writer.WriteMessages(
		context.Background(),
		kafka.Message{
			Key:   []byte(events.UserAuthorizedEventName),
			Value: payload,
		},
	)
}

func (controller *ApiController) buildState(authOptionsClaims AuthOptionsClaims) (state string, err error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, &authOptionsClaims)
	state, err = token.SignedString(controller.config.jwtSecretKey)
	return
}

var jwtParser = jwt.NewParser(jwt.WithValidMethods([]string{"HS512"}))

func (controller *ApiController) parseState(state string) (claims AuthOptionsClaims, err error) {
	_, err = jwtParser.ParseWithClaims(
		state, &claims,
		func(token *jwt.Token) (interface{}, error) {
			return controller.config.jwtSecretKey, nil
		},
	)

	return claims, err
}
