package main

import (
	"fmt"
	"github.com/berejant/go-kneu"
	"github.com/gin-gonic/gin"
	"github.com/segmentio/kafka-go"
	"io"
	"net/http"
	"os"
)

const ExitCodeMainError = 1

type httpInterface interface {
	ListenAndServe(addr string, handler http.Handler) error
}

func main() {
	os.Exit(handleExitError(os.Stderr, runApp(os.Stdout, http.ListenAndServe)))
}

func runApp(out io.Writer, listenAndServe func(string, http.Handler) error) error {
	envFilename := ""
	if _, err := os.Stat(".env"); err == nil {
		envFilename = ".env"
	}

	config, err := loadConfig(envFilename)
	if err != nil {
		return err
	}

	gin.SetMode(gin.ReleaseMode)

	apiController := &ApiController{
		out:    out,
		config: config,
		writer: &kafka.Writer{
			Addr:     kafka.TCP(config.kafkaHost),
			Topic:    "authorized_users",
			Balancer: &kafka.LeastBytes{},
		},
		oauthClient: &kneu.OauthClient{
			BaseUri:      config.kneuBaseUri,
			ClientId:     config.kneuClientId,
			ClientSecret: config.kneuClientSecret,
		},
		apiClientFactory: func(token string) kneu.ApiClientInterface {
			return &kneu.ApiClient{
				BaseUri:     config.kneuBaseUri,
				AccessToken: token,
			}
		},
	}

	apiController.apiClientFactory("test")

	return listenAndServe(
		config.listenAddress, apiController.setupRouter(),
	)
}

func handleExitError(errStream io.Writer, err error) int {
	if err != nil {
		_, _ = fmt.Fprintln(errStream, err)
	}

	if err != nil {
		return ExitCodeMainError
	}

	return 0
}
