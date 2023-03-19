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
	kneuClientId     int
	kneuClientSecret string

	jwtSecretKey []byte

	appSecret string
}

func loadConfig(envFilename string) (Config, error) {
	if envFilename != "" {
		err := godotenv.Load(envFilename)
		if err != nil {
			return Config{}, errors.New(fmt.Sprintf("Error loading %s file: %s", envFilename, err))
		}
	}

	kneuClientId, err := strconv.Atoi(os.Getenv("KNEU_CLIENT_ID"))
	if err != nil || kneuClientId < 1 {
		return Config{}, errors.New(fmt.Sprintf("Wrong KNEU client (%d) ID %s", kneuClientId, err))
	}

	config := Config{
		publicUrl:     strings.TrimRight(os.Getenv("PUBLIC_URL"), "/"),
		listenAddress: os.Getenv("LISTEN"),
		kafkaHost:     os.Getenv("KAFKA_HOST"),

		kneuBaseUri:      os.Getenv("KNEU_BASE_URI"),
		kneuClientId:     kneuClientId,
		kneuClientSecret: os.Getenv("KNEU_CLIENT_SECRET"),

		jwtSecretKey: []byte(os.Getenv("JWT_SECRET_KEY")),

		appSecret: os.Getenv("APP_SECRET"),
	}

	if config.publicUrl == "" {
		return Config{}, errors.New("empty PUBLIC_URL")
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
