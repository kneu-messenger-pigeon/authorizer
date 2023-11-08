package main

import (
	"authorizer/dto"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/berejant/go-kneu"
	"github.com/golang-jwt/jwt/v5"
	"github.com/kneu-messenger-pigeon/events"
	"github.com/kneu-messenger-pigeon/events/mocks"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPingRoute(t *testing.T) {
	router := (&ApiController{}).setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/healthcheck", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "health", w.Body.String())
}

func TestCloseHtml(t *testing.T) {
	router := (&ApiController{}).setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/close.html", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "location.replace(\"https://kneu.edu.ua/\");")
}

func TestGetAuthUrl(t *testing.T) {
	config := Config{
		publicUrl:        "https://pigeon.com",
		listenAddress:    "",
		kafkaHost:        "",
		kneuBaseUri:      "",
		kneuClientId:     0,
		kneuClientSecret: "",
		jwtSecretKey:     nil,
		appSecret:        "test-secret",
	}

	t.Run("success", func(t *testing.T) {
		expectedOauthUrl := "https://auth.kneu.edu.ua/oauth?response_type=code&client_id=0&redirect_uri=https%3A%2F%2Fpigeon.com%2Fcomplete&_state_"
		redirectUrl := "https://example.com"
		client := "telegram"
		clientUserId := "99"

		var receivedState string

		oauthClient := kneu.NewMockOauthClientInterface(t)
		oauthClient.On(
			"GetOauthUrl", config.publicUrl+"/complete",
			mock.MatchedBy(func(state string) bool {
				receivedState = state
				return true
			}),
		).Return(expectedOauthUrl)

		router := (&ApiController{
			out:         &bytes.Buffer{},
			oauthClient: oauthClient,
			config:      config,
		}).setupRouter()

		startTime := time.Now().Truncate(jwt.TimePrecision)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/url", strings.NewReader("client="+client+"&client_user_id="+clientUserId+"&redirect_uri="+redirectUrl))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("pigeon", config.appSecret)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		response := dto.GetAuthUrlResponse{}
		err := json.NewDecoder(w.Body).Decode(&response)
		assert.NoError(t, err)

		assert.Equal(t, expectedOauthUrl, response.AuthUrl)

		authOptionsClaims := dto.AuthOptionsClaims{}
		_, err = jwtParser.ParseWithClaims(
			receivedState, &authOptionsClaims,
			func(token *jwt.Token) (interface{}, error) {
				return config.jwtSecretKey, nil
			},
		)

		assert.Equal(t, redirectUrl, authOptionsClaims.RedirectUri)
		assert.Equal(t, client, authOptionsClaims.Client)
		assert.Equal(t, clientUserId, authOptionsClaims.ClientUserId)

		assert.GreaterOrEqual(t, response.ExpireAt, startTime.Add(stateLifetime))
		assert.LessOrEqual(t, response.ExpireAt, time.Now().Add(stateLifetime))
		assert.Equal(t, authOptionsClaims.ExpiresAt.Time, response.ExpireAt)
	})

	t.Run("error", func(t *testing.T) {

		router := (&ApiController{
			out:    &bytes.Buffer{},
			config: config,
		}).setupRouter()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/url", strings.NewReader(""))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("pigeon", config.appSecret)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "{\"error\":\"Wrong request data\"}", w.Body.String())
	})
}

func TestCompleteAuth(t *testing.T) {
	config := Config{
		publicUrl:        "https://pigeon.com",
		listenAddress:    "",
		kafkaHost:        "",
		kneuBaseUri:      "",
		kneuClientId:     0,
		kneuClientSecret: "",
		jwtSecretKey:     nil,
		appSecret:        "test-secret",
	}

	t.Run("success", func(t *testing.T) {
		t.Run("success custom redirect", func(t *testing.T) {
			client := "telegram"
			clientUserId := "999"
			code := "qwerty1234"
			userId := uint(999)
			studentId := uint(123)

			oauthClient := kneu.NewMockOauthClientInterface(t)
			apiClient := kneu.NewMockApiClientInterface(t)

			writer := mocks.NewWriterInterface(t)

			tokenResponse := kneu.OauthTokenResponse{
				AccessToken: "test-access-token",
				TokenType:   "Bearer",
				ExpiresIn:   7200,
				UserId:      userId,
			}

			oauthClient.On("GetOauthToken", config.publicUrl+"/complete", code).Return(tokenResponse, nil)

			userMeResponse := kneu.UserMeResponse{
				Id:           userId,
				Email:        "example@kneu.edu.ua",
				Name:         "Коваль Валера Павлович",
				LastName:     "Коваль",
				FirstName:    "Валера",
				MiddleName:   "Павлович",
				Type:         "student",
				StudentId:    studentId,
				GroupId:      12,
				Gender:       "male",
				TeacherId:    0,
				DepartmentId: 0,
			}
			apiClient.On("GetUserMe").Return(userMeResponse, nil)

			payload, _ := json.Marshal(events.UserAuthorizedEvent{
				Client:       client,
				ClientUserId: clientUserId,
				StudentId:    studentId,
				LastName:     "Коваль",
				FirstName:    "Валера",
				MiddleName:   "Павлович",
				Gender:       events.Male,
			})

			expectedMessage := kafka.Message{
				Key:   []byte(events.UserAuthorizedEventName),
				Value: payload,
			}
			writer.On("WriteMessages", context.Background(), expectedMessage).Return(nil)

			controller := &ApiController{
				out:         &bytes.Buffer{},
				writer:      writer,
				config:      config,
				oauthClient: oauthClient,
				apiClientFactory: func(token string) kneu.ApiClientInterface {
					assert.Equal(t, tokenResponse.AccessToken, token)
					return apiClient
				},
				countCache: NewCountCache(1),
			}

			router := (controller).setupRouter()

			authOptionsClaims := dto.AuthOptionsClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    "pigeonAuthorizer",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(stateLifetime)),
				},
				Client:       client,
				ClientUserId: clientUserId,
				RedirectUri:  "https://example.com/redirect",
				KneuUserId:   0,
			}

			state, _ := controller.buildState(authOptionsClaims)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/complete?code="+code+"&state="+state, nil)

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusFound, w.Code)
			assert.Equal(t, authOptionsClaims.RedirectUri, w.Header().Get("Location"))
		})

		t.Run("success default redirect", func(t *testing.T) {
			client := "telegram"
			clientUserId := "999"
			code := "qwerty1234"
			userId := uint(999)
			studentId := uint(123)

			oauthClient := kneu.NewMockOauthClientInterface(t)
			apiClient := kneu.NewMockApiClientInterface(t)

			writer := mocks.NewWriterInterface(t)

			tokenResponse := kneu.OauthTokenResponse{
				AccessToken: "test-access-tokem",
				TokenType:   "Bearer",
				ExpiresIn:   7200,
				UserId:      userId,
			}

			oauthClient.On("GetOauthToken", config.publicUrl+"/complete", code).Return(tokenResponse, nil)

			userMeResponse := kneu.UserMeResponse{
				Id:           userId,
				Email:        "example@kneu.edu.ua",
				Name:         "Коваль Валера Павлович",
				LastName:     "Коваль",
				FirstName:    "Валера",
				MiddleName:   "Павлович",
				Type:         "student",
				StudentId:    studentId,
				GroupId:      12,
				Gender:       "female",
				TeacherId:    0,
				DepartmentId: 0,
			}
			apiClient.On("GetUserMe").Return(userMeResponse, nil)

			payload, _ := json.Marshal(events.UserAuthorizedEvent{
				Client:       client,
				ClientUserId: clientUserId,
				StudentId:    studentId,
				LastName:     "Коваль",
				FirstName:    "Валера",
				MiddleName:   "Павлович",
				Gender:       events.Female,
			})

			expectedMessage := kafka.Message{
				Key:   []byte(events.UserAuthorizedEventName),
				Value: payload,
			}
			writer.On("WriteMessages", context.Background(), expectedMessage).Return(nil)

			controller := &ApiController{
				out:         &bytes.Buffer{},
				writer:      writer,
				config:      config,
				oauthClient: oauthClient,
				apiClientFactory: func(token string) kneu.ApiClientInterface {
					assert.Equal(t, tokenResponse.AccessToken, token)
					return apiClient
				},
				countCache: NewCountCache(1),
			}

			router := (controller).setupRouter()

			authOptionsClaims := dto.AuthOptionsClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    "pigeonAuthorizer",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(stateLifetime)),
				},
				Client:       client,
				ClientUserId: clientUserId,
				RedirectUri:  "",
				KneuUserId:   0,
			}

			state, _ := controller.buildState(authOptionsClaims)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/complete?code="+code+"&state="+state, nil)

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusFound, w.Code)
			assert.Equal(t, config.publicUrl+"/close.html", w.Header().Get("Location"))
		})

		t.Run("success_admin", func(t *testing.T) {
			client := "telegram"
			clientUserId := "999"
			code := "qwerty1234"

			oauthClient := kneu.NewMockOauthClientInterface(t)

			tokenResponse := kneu.OauthTokenResponse{
				AccessToken: "test-access-tokem",
				TokenType:   "Bearer",
				ExpiresIn:   7200,
				UserId:      adminUserid,
			}

			oauthClient.On("GetOauthToken", config.publicUrl+"/complete", code).Return(tokenResponse, nil)

			controller := &ApiController{
				out:         &bytes.Buffer{},
				config:      config,
				oauthClient: oauthClient,
				countCache:  NewCountCache(1),
			}

			router := (controller).setupRouter()

			authOptionsClaims := dto.AuthOptionsClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    "pigeonAuthorizer",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(stateLifetime)),
				},
				Client:       client,
				ClientUserId: clientUserId,
				RedirectUri:  "",
				KneuUserId:   0,
			}

			state, _ := controller.buildState(authOptionsClaims)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/complete?code="+code+"&state="+state, nil)

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			body := w.Body.String()

			assert.Contains(t, body, "state")
			assert.Contains(t, body, `action="`+config.publicUrl+`/admin"`)
			assert.Contains(t, body, `name="student_id"`)
		})
	})

	t.Run("error", func(t *testing.T) {
		t.Run("error_not_student", func(t *testing.T) {
			client := "telegram"
			clientUserId := "999"
			finalRedirectUri := "http://example.com"
			code := "qwerty1234"
			userId := uint(999)

			oauthClient := kneu.NewMockOauthClientInterface(t)
			apiClient := kneu.NewMockApiClientInterface(t)

			tokenResponse := kneu.OauthTokenResponse{
				AccessToken: "test-access-tokem",
				TokenType:   "Bearer",
				ExpiresIn:   7200,
				UserId:      userId,
			}

			oauthClient.On("GetOauthToken", config.publicUrl+"/complete", code).Return(tokenResponse, nil)

			userMeResponse := kneu.UserMeResponse{
				Id:           userId,
				Email:        "example@kneu.edu.ua",
				Name:         "Коваль Валера Павлович",
				LastName:     "Коваль",
				FirstName:    "Валера",
				MiddleName:   "Павлович",
				Type:         "teacher",
				StudentId:    0,
				GroupId:      12,
				Gender:       "male",
				TeacherId:    655,
				DepartmentId: 0,
			}
			apiClient.On("GetUserMe").Return(userMeResponse, nil)

			oauthClient.On("GetOauthToken", config.publicUrl+"/complete", code).Return(tokenResponse, nil)

			controller := &ApiController{
				out:         &bytes.Buffer{},
				config:      config,
				oauthClient: oauthClient,
				apiClientFactory: func(token string) kneu.ApiClientInterface {
					assert.Equal(t, tokenResponse.AccessToken, token)
					return apiClient
				},
				countCache: NewCountCache(1),
			}

			router := (controller).setupRouter()

			authOptionsClaims := dto.AuthOptionsClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    "pigeonAuthorizer",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(stateLifetime)),
				},
				Client:       client,
				ClientUserId: clientUserId,
				RedirectUri:  finalRedirectUri,
				KneuUserId:   0,
			}

			state, _ := controller.buildState(authOptionsClaims)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/complete?code="+code+"&state="+state, nil)

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "Не вдалося завершити авторизацію")
			assert.Contains(t, w.Body.String(), "Вам потрібно використати особистий кабінет студента")
		})

		t.Run("error_expired_state", func(t *testing.T) {
			client := "telegram"
			clientUserId := "999"
			finalRedirectUri := "http://example.com"
			code := "qwerty1234"

			oauthClient := kneu.NewMockOauthClientInterface(t)
			apiClient := kneu.NewMockApiClientInterface(t)

			tokenResponse := kneu.OauthTokenResponse{}

			out := &bytes.Buffer{}
			controller := &ApiController{
				out:         out,
				config:      config,
				oauthClient: oauthClient,
				apiClientFactory: func(token string) kneu.ApiClientInterface {
					assert.Equal(t, tokenResponse.AccessToken, token)
					return apiClient
				},
				countCache: NewCountCache(1),
			}

			router := (controller).setupRouter()

			authOptionsClaims := dto.AuthOptionsClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    "pigeonAuthorizer",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour * 6)),
				},
				Client:       client,
				ClientUserId: clientUserId,
				RedirectUri:  finalRedirectUri,
				KneuUserId:   0,
			}

			state, _ := controller.buildState(authOptionsClaims)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/complete?code="+code+"&state="+state, nil)

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "Не вдалося завершити авторизацію")
			assert.Contains(t, w.Body.String(), "Посилання для авторизації через сайт КНЕУ отримане через бот недійсне. Отримайте нове посилання надіславши боту команду /start")
		})

		t.Run("error_wrong_state", func(t *testing.T) {
			code := "qwerty1234"

			oauthClient := kneu.NewMockOauthClientInterface(t)
			apiClient := kneu.NewMockApiClientInterface(t)

			tokenResponse := kneu.OauthTokenResponse{}

			out := &bytes.Buffer{}
			controller := &ApiController{
				out:         out,
				config:      config,
				oauthClient: oauthClient,
				apiClientFactory: func(token string) kneu.ApiClientInterface {
					assert.Equal(t, tokenResponse.AccessToken, token)
					return apiClient
				},
				countCache: NewCountCache(1),
			}

			router := (controller).setupRouter()

			state := "ahahahahahahahah"

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/complete?code="+code+"&state="+state, nil)

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "Не вдалося завершити авторизацію")
			assert.Contains(t, w.Body.String(), "Некорректне посилання для авторизації через сайт КНЕУ")
			assert.Contains(t, out.String(), "Failed to parse state")
		})

		t.Run("error_fail_to_get_token", func(t *testing.T) {
			client := "telegram"
			clientUserId := "999"
			finalRedirectUri := "http://example.com"
			code := "qwerty1234"

			oauthClient := kneu.NewMockOauthClientInterface(t)
			apiClient := kneu.NewMockApiClientInterface(t)

			tokenResponse := kneu.OauthTokenResponse{}

			expectedError := errors.New("dummy expectedError")

			oauthClient.On("GetOauthToken", config.publicUrl+"/complete", code).Return(tokenResponse, expectedError)

			out := &bytes.Buffer{}
			controller := &ApiController{
				out:         out,
				config:      config,
				oauthClient: oauthClient,
				apiClientFactory: func(token string) kneu.ApiClientInterface {
					assert.Equal(t, tokenResponse.AccessToken, token)
					return apiClient
				},
				countCache: NewCountCache(1),
			}

			router := (controller).setupRouter()

			authOptionsClaims := dto.AuthOptionsClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    "pigeonAuthorizer",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(stateLifetime)),
				},
				Client:       client,
				ClientUserId: clientUserId,
				RedirectUri:  finalRedirectUri,
				KneuUserId:   0,
			}

			state, _ := controller.buildState(authOptionsClaims)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/complete?code="+code+"&state="+state, nil)

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "Не вдалося завершити авторизацію")
			assert.Contains(t, w.Body.String(), "Помилка отримання токена")
			assert.Contains(t, out.String(), expectedError.Error())
		})

		t.Run("error_fail_to_get_user_me", func(t *testing.T) {
			client := "telegram"
			clientUserId := "999"
			finalRedirectUri := "http://example.com"
			code := "qwerty1234"
			userId := uint(999)

			oauthClient := kneu.NewMockOauthClientInterface(t)
			apiClient := kneu.NewMockApiClientInterface(t)

			tokenResponse := kneu.OauthTokenResponse{
				AccessToken: "test-access-token",
				TokenType:   "Bearer",
				ExpiresIn:   7200,
				UserId:      userId,
			}

			oauthClient.On("GetOauthToken", config.publicUrl+"/complete", code).Return(tokenResponse, nil)

			expectedError := errors.New("dummy expectedError")
			userMeResponse := kneu.UserMeResponse{}
			apiClient.On("GetUserMe").Return(userMeResponse, expectedError)

			out := &bytes.Buffer{}
			controller := &ApiController{
				out:         out,
				config:      config,
				oauthClient: oauthClient,
				apiClientFactory: func(token string) kneu.ApiClientInterface {
					assert.Equal(t, tokenResponse.AccessToken, token)
					return apiClient
				},
				countCache: NewCountCache(1),
			}

			router := (controller).setupRouter()

			authOptionsClaims := dto.AuthOptionsClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    "pigeonAuthorizer",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(stateLifetime)),
				},
				Client:       client,
				ClientUserId: clientUserId,
				RedirectUri:  finalRedirectUri,
				KneuUserId:   0,
			}

			state, _ := controller.buildState(authOptionsClaims)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/complete?code="+code+"&state="+state, nil)

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "Не вдалося завершити авторизацію")
			assert.Contains(t, w.Body.String(), "Помилка отримання даних користувача")
			assert.Contains(t, out.String(), expectedError.Error())
		})

		t.Run("error_fail_finish_auth", func(t *testing.T) {
			client := "telegram"
			clientUserId := "999"
			finalRedirectUri := "http://example.com"
			code := "qwerty1234"
			studentId := uint(123)
			userId := uint(999)

			oauthClient := kneu.NewMockOauthClientInterface(t)
			apiClient := kneu.NewMockApiClientInterface(t)
			writer := mocks.NewWriterInterface(t)

			tokenResponse := kneu.OauthTokenResponse{
				AccessToken: "test-access-token",
				TokenType:   "Bearer",
				ExpiresIn:   7200,
				UserId:      userId,
			}

			oauthClient.On("GetOauthToken", config.publicUrl+"/complete", code).Return(tokenResponse, nil)

			userMeResponse := kneu.UserMeResponse{
				Id:           userId,
				Email:        "example@kneu.edu.ua",
				Name:         "Коваль Валера Павлович",
				LastName:     "Коваль",
				FirstName:    "Валера",
				MiddleName:   "Павлович",
				Type:         "student",
				StudentId:    studentId,
				GroupId:      12,
				Gender:       "male",
				TeacherId:    0,
				DepartmentId: 0,
			}
			apiClient.On("GetUserMe").Return(userMeResponse, nil)

			payload, _ := json.Marshal(events.UserAuthorizedEvent{
				Client:       client,
				ClientUserId: clientUserId,
				StudentId:    studentId,
				LastName:     "Коваль",
				FirstName:    "Валера",
				MiddleName:   "Павлович",
				Gender:       events.Male,
			})

			expectedMessage := kafka.Message{
				Key:   []byte(events.UserAuthorizedEventName),
				Value: payload,
			}

			writerError := errors.New("dummy error")
			writer.On("WriteMessages", context.Background(), expectedMessage).Return(writerError)

			out := &bytes.Buffer{}

			controller := &ApiController{
				out:         out,
				config:      config,
				oauthClient: oauthClient,
				writer:      writer,
				apiClientFactory: func(token string) kneu.ApiClientInterface {
					assert.Equal(t, tokenResponse.AccessToken, token)
					return apiClient
				},
				countCache: NewCountCache(1),
			}

			router := (controller).setupRouter()

			authOptionsClaims := dto.AuthOptionsClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    "pigeonAuthorizer",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(stateLifetime)),
				},
				Client:       client,
				ClientUserId: clientUserId,
				RedirectUri:  finalRedirectUri,
				KneuUserId:   0,
			}

			state, _ := controller.buildState(authOptionsClaims)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/complete?code="+code+"&state="+state, nil)

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "Не вдалося завершити авторизацію")
			assert.Contains(t, w.Body.String(), "Помилка завершення авторизації")
			assert.Contains(t, out.String(), writerError.Error())
		})

		t.Run("error_admin", func(t *testing.T) {
			client := "telegram"
			clientUserId := "999"
			code := "qwerty1234"

			oauthClient := kneu.NewMockOauthClientInterface(t)

			tokenResponse := kneu.OauthTokenResponse{
				AccessToken: "test-access-tokem",
				TokenType:   "Bearer",
				ExpiresIn:   7200,
				UserId:      adminUserid,
			}

			oauthClient.On("GetOauthToken", config.publicUrl+"/complete", code).Return(tokenResponse, nil)

			controller := &ApiController{
				out:         &bytes.Buffer{},
				config:      config,
				oauthClient: oauthClient,
				countCache:  NewCountCache(1),
			}

			router := (controller).setupRouter()

			authOptionsClaims := dto.AuthOptionsClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    "pigeonAuthorizer",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(stateLifetime)),
				},
				Client:       client,
				ClientUserId: clientUserId,
				RedirectUri:  "",
				KneuUserId:   0,
			}

			state, _ := controller.buildState(authOptionsClaims)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/complete?code="+code+"&state="+state, nil)

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			body := w.Body.String()

			assert.Contains(t, body, "state")
			assert.Contains(t, body, `action="`+config.publicUrl+`/admin"`)
			assert.Contains(t, body, `name="student_id"`)
		})
	})

}

func TestCompleteAdminAuth(t *testing.T) {
	config := Config{
		publicUrl:        "https://pigeon.com",
		listenAddress:    "",
		kafkaHost:        "",
		kneuBaseUri:      "",
		kneuClientId:     0,
		kneuClientSecret: "",
		jwtSecretKey:     nil,
		appSecret:        "test-secret",
	}

	t.Run("success", func(t *testing.T) {
		client := "telegram"
		clientUserId := "999"
		studentId := uint(123)

		writer := mocks.NewWriterInterface(t)

		payload, _ := json.Marshal(events.UserAuthorizedEvent{
			Client:       client,
			ClientUserId: clientUserId,
			StudentId:    studentId,
			LastName:     "Адмін",
			FirstName:    "Адмін#123",
			MiddleName:   "Адмін",
			Gender:       events.UnknownGender,
		})

		expectedMessage := kafka.Message{
			Key:   []byte(events.UserAuthorizedEventName),
			Value: payload,
		}
		writer.On("WriteMessages", context.Background(), expectedMessage).Return(nil)

		controller := &ApiController{
			out:    &bytes.Buffer{},
			writer: writer,
			config: config,
		}

		router := (controller).setupRouter()

		authOptionsClaims := dto.AuthOptionsClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    "pigeonAuthorizer",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(stateLifetime)),
			},
			Client:       client,
			ClientUserId: clientUserId,
			RedirectUri:  "",
			KneuUserId:   adminUserid,
		}

		state, _ := controller.buildState(authOptionsClaims)

		w := httptest.NewRecorder()

		postData := "state=" + state + "&student_id=" + strconv.FormatUint(uint64(studentId), 10)
		req, _ := http.NewRequest(http.MethodPost, "/admin", strings.NewReader(postData))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, config.publicUrl+"/close.html", w.Header().Get("Location"))
	})

	t.Run("empty_student_id", func(t *testing.T) {
		client := "telegram"
		clientUserId := "999"

		controller := &ApiController{
			out:    &bytes.Buffer{},
			config: config,
		}

		router := (controller).setupRouter()

		authOptionsClaims := dto.AuthOptionsClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    "pigeonAuthorizer",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(stateLifetime)),
			},
			Client:       client,
			ClientUserId: clientUserId,
			RedirectUri:  "",
			KneuUserId:   adminUserid,
		}

		state, _ := controller.buildState(authOptionsClaims)

		w := httptest.NewRecorder()

		req, _ := http.NewRequest(http.MethodPost, "/admin", strings.NewReader("state="+state+"&student_id=0"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		router.ServeHTTP(w, req)

		body := w.Body.String()

		assert.Contains(t, body, "state")
		assert.Contains(t, body, `action="`+config.publicUrl+`/admin"`)
		assert.Contains(t, body, `name="student_id"`)
	})

	t.Run("error_state", func(t *testing.T) {
		controller := &ApiController{
			out:    &bytes.Buffer{},
			config: config,
		}

		router := (controller).setupRouter()

		w := httptest.NewRecorder()

		req, _ := http.NewRequest(http.MethodPost, "/admin", strings.NewReader("state=wrong-test&student_id=12"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Не вдалося завершити авторизацію")
	})
}
