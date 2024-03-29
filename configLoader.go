package main

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/joho/godotenv"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	publicUrl     string
	listenAddress string
	kafkaHost     string

	kneuBaseUri      string
	kneuClientId     uint
	kneuClientSecret string

	jwtSecretKey      []byte
	authStateLifetime time.Duration

	appSecret string
}

func loadConfig(envFilename string) (Config, error) {
	if envFilename != "" {
		err := godotenv.Load(envFilename)
		if err != nil {
			return Config{}, errors.New(fmt.Sprintf("Error loading %s file: %s", envFilename, err))
		}
	}

	kneuClientId, err := strconv.ParseUint(os.Getenv("KNEU_CLIENT_ID"), 10, 0)
	if err != nil || kneuClientId < 1 {
		return Config{}, errors.New(fmt.Sprintf("Wrong KNEU client (%d) ID %s", kneuClientId, err))
	}

	authStateLifetime, err := time.ParseDuration(os.Getenv("AUTH_STATE_LIFETIME"))
	if authStateLifetime < 1 {
		return Config{}, errors.New(fmt.Sprintf("Wrong AUTH_STATE_LIFETIME %s", err))
	}

	config := Config{
		publicUrl:     strings.TrimRight(os.Getenv("AUTHORIZER_PUBLIC_URL"), "/"),
		listenAddress: os.Getenv("LISTEN"),
		kafkaHost:     os.Getenv("KAFKA_HOST"),

		kneuBaseUri:      os.Getenv("KNEU_BASE_URI"),
		kneuClientId:     uint(kneuClientId),
		kneuClientSecret: os.Getenv("KNEU_CLIENT_SECRET"),

		jwtSecretKey:      []byte(os.Getenv("JWT_SECRET_KEY")),
		authStateLifetime: authStateLifetime,

		appSecret: os.Getenv("APP_SECRET"),
	}

	if config.publicUrl == "" {
		return Config{}, errors.New("empty AUTHORIZER_PUBLIC_URL")
	}

	if config.listenAddress == "" {
		return Config{}, errors.New("empty LISTEN")
	}

	if config.kafkaHost == "" {
		return Config{}, errors.New("empty KAFKA_HOST")
	}

	if config.kneuClientSecret == "" {
		return Config{}, errors.New("empty KNEU_CLIENT_SECRET")
	}

	if len(config.jwtSecretKey) == 0 {
		now := time.Now().Unix()
		key := md5.Sum([]byte(
			strconv.FormatInt(now, 16) + config.kneuClientSecret + strconv.FormatInt(now, 36),
		))

		config.jwtSecretKey = make([]byte, len(key)*2)
		hex.Encode(config.jwtSecretKey, key[:])
	}

	return config, nil
}
