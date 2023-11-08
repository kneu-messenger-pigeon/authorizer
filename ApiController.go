package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/berejant/go-kneu"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/kneu-messenger-pigeon/authorizer/dto"
	"github.com/kneu-messenger-pigeon/events"
	"github.com/segmentio/kafka-go"
	"html"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"time"
)

const adminUserid = 1

var stateLifetime = time.Hour * 6

//go:embed templates/*.html
var templates embed.FS

type ApiController struct {
	out              io.Writer
	config           Config
	writer           events.WriterInterface
	oauthClient      kneu.OauthClientInterface
	apiClientFactory func(token string) kneu.ApiClientInterface

	oauthRedirectUrl string

	countCache *countCache
}

func (controller *ApiController) setupRouter() *gin.Engine {
	router := gin.New()

	completeUri := "/complete"
	controller.oauthRedirectUrl = controller.config.publicUrl + completeUri

	router.SetHTMLTemplate(
		template.Must(template.New("").ParseFS(templates, "templates/*.html")),
	)

	router.POST("/url", gin.BasicAuth(gin.Accounts{
		"pigeon": controller.config.appSecret,
	}), controller.getAuthUrl)

	router.GET(completeUri, controller.completeAuth)
	router.POST("/admin", controller.completeAdminAuth)

	router.StaticFile("/close.html", "./templates/close.html")

	router.Any("/healthcheck", func(c *gin.Context) {
		c.String(http.StatusOK, "health")
	})

	return router
}

func (controller *ApiController) getAuthUrl(c *gin.Context) {
	getAuthUrlRequestsTotal.Inc()

	var err error
	var state string

	authOptionsClaims := dto.AuthOptionsClaims{}
	err = c.Bind(&authOptionsClaims)
	expireAt := time.Now().Add(stateLifetime).Truncate(jwt.TimePrecision)
	if err == nil {
		authOptionsClaims.KneuUserId = 0
		authOptionsClaims.Issuer = "pigeonAuthorizer"
		authOptionsClaims.ExpiresAt = jwt.NewNumericDate(expireAt)

		state, err = controller.buildState(authOptionsClaims)
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Wrong request data"})
	} else {
		response := dto.GetAuthUrlResponse{
			AuthUrl:  controller.oauthClient.GetOauthUrl(controller.oauthRedirectUrl, state),
			ExpireAt: expireAt,
		}

		c.JSON(http.StatusOK, &response)
	}
}

func (controller *ApiController) completeAuth(c *gin.Context) {

	var authOptionsClaims dto.AuthOptionsClaims
	var tokenResponse kneu.OauthTokenResponse
	var userMeResponse kneu.UserMeResponse

	code := c.Query("code")
	state := c.Query("state")

	stateReceivedCount := controller.countCache.Get(&state)
	controller.countCache.Set(&state, stateReceivedCount+1)
	if stateReceivedCount == 0 {
		completeAuthRequestsTotal.Inc()
	}

	authOptionsClaims, err := controller.parseState(state)
	if errors.Is(err, jwt.ErrTokenExpired) {
		controller.errorResponse(c, "Посилання для авторизації через сайт КНЕУ отримане через бот недійсне. Отримайте нове посилання надіславши боту команду /start")
		authErrorExpiredStateTotal.Inc()
		return
	} else if err != nil {
		controller.errorResponse(c, "Некорректне посилання для авторизації через сайт КНЕУ. Отримайте нове посилання надіславши боту команду /start")
		fmt.Fprintf(controller.out, "Failed to parse state: %s\n", err.Error())
		authErrorWrongStateTotal.Inc()
		return
	}

	tokenResponse, err = controller.oauthClient.GetOauthToken(controller.oauthRedirectUrl, code)
	if err != nil {
		fmt.Fprintf(controller.out, "Failed to get token: %s\n", err.Error())
		controller.errorResponse(c, "Помилка отримання токена")
		authErrorFailGetTokenTotal.Inc()
		return
	}

	if err == nil && tokenResponse.UserId == adminUserid {
		authOptionsClaims.KneuUserId = tokenResponse.UserId

		state, err = controller.buildState(authOptionsClaims)
		controller.responseWithAdminAuthFrom(c, state)
		return
	}

	userMeResponse, err = controller.apiClientFactory(tokenResponse.AccessToken).GetUserMe()
	if err != nil {
		fmt.Fprintf(controller.out, "Failed to get user me: %s\n", err.Error())
		controller.errorResponse(c, "Помилка отримання даних користувача")
		authErrorFailGetUserTotal.Inc()
		return
	}

	if userMeResponse.Type != "student" {
		controller.errorResponse(c, "Вам потрібно використати особистий кабінет студента")
		return
	}

	err = controller.finishAuthorization(authOptionsClaims, dto.Student{
		Id:         userMeResponse.StudentId,
		LastName:   userMeResponse.LastName,
		FirstName:  userMeResponse.FirstName,
		MiddleName: userMeResponse.MiddleName,
		Gender:     events.GenderFromString(userMeResponse.Gender),
	})

	if err != nil {
		fmt.Fprintf(controller.out, "Failed to auth user: %s\n", err.Error())
		controller.errorResponse(c, "Помилка завершення авторизації")
		authErrorFailFinishAuthTotal.Inc()
		return
	} else {
		controller.successRedirect(c, authOptionsClaims)
	}
}

func (controller *ApiController) successRedirect(c *gin.Context, claims dto.AuthOptionsClaims) {
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
		"actionUrl": controller.config.publicUrl + "/admin",
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
			err = controller.finishAuthorization(authOptionsClaims, dto.Student{
				Id:         uint(studentId),
				LastName:   "Адмін",
				FirstName:  "Адмін#" + strconv.FormatUint(studentId, 10),
				MiddleName: "Адмін",
				Gender:     events.UnknownGender,
			})
			if err == nil {
				controller.successRedirect(c, authOptionsClaims)
				return
			}
		}
	}

	controller.errorResponse(c, err.Error())
}

func (controller *ApiController) finishAuthorization(claims dto.AuthOptionsClaims, student dto.Student) error {
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

func (controller *ApiController) buildState(authOptionsClaims dto.AuthOptionsClaims) (state string, err error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, &authOptionsClaims)
	state, err = token.SignedString(controller.config.jwtSecretKey)
	return
}

var jwtParser = jwt.NewParser(jwt.WithValidMethods([]string{"HS512"}))

func (controller *ApiController) parseState(state string) (claims dto.AuthOptionsClaims, err error) {
	_, err = jwtParser.ParseWithClaims(
		state, &claims,
		func(token *jwt.Token) (interface{}, error) {
			return controller.config.jwtSecretKey, nil
		},
	)

	return claims, err
}

func (controller *ApiController) errorResponse(c *gin.Context, errMessage string) {
	c.HTML(http.StatusBadRequest, "error.html", gin.H{
		"error": html.EscapeString(errMessage),
	})
}
