package main

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"strconv"
	"testing"
)

var expectedConfig = Config{
	publicUrl:     "http://localhost:9080",
	listenAddress: ":9080",
	kafkaHost:     "KAFKA:9999",

	kneuBaseUri:      "https://auth.kneu.test/",
	kneuClientId:     99,
	kneuClientSecret: "testClientSecret",

	jwtSecretKey: []byte{'t', 'e', 's', 't'},

	appSecret: "test_Secret_test123",
}

func TestLoadConfigFromEnvVars(t *testing.T) {
	t.Run("FromEnvVars", func(t *testing.T) {
		_ = os.Setenv("AUTHORIZER_PUBLIC_URL", expectedConfig.publicUrl)
		_ = os.Setenv("LISTEN", expectedConfig.listenAddress)
		_ = os.Setenv("KAFKA_HOST", expectedConfig.kafkaHost)
		_ = os.Setenv("KNEU_BASE_URI", expectedConfig.kneuBaseUri)
		_ = os.Setenv("KNEU_CLIENT_ID", strconv.Itoa(int(expectedConfig.kneuClientId)))
		_ = os.Setenv("KNEU_CLIENT_SECRET", expectedConfig.kneuClientSecret)
		_ = os.Setenv("JWT_SECRET_KEY", string(expectedConfig.jwtSecretKey))
		_ = os.Setenv("APP_SECRET", string(expectedConfig.appSecret))

		config, err := loadConfig("")

		assert.NoErrorf(t, err, "got unexpected error %s", err)
		assertConfig(t, expectedConfig, config)
		assert.Equalf(t, expectedConfig, config, "Expected for %v, actual: %v", expectedConfig, config)
	})

	t.Run("FromFile", func(t *testing.T) {
		var envFileContent string

		envFileContent += fmt.Sprintf("AUTHORIZER_PUBLIC_URL=%s\n", expectedConfig.publicUrl)
		envFileContent += fmt.Sprintf("LISTEN=%s\n", expectedConfig.listenAddress)
		envFileContent += fmt.Sprintf("KAFKA_HOST=%s\n", expectedConfig.kafkaHost)

		envFileContent += fmt.Sprintf("KNEU_BASE_URI=%s\n", expectedConfig.kneuBaseUri)
		envFileContent += fmt.Sprintf("KNEU_CLIENT_ID=%d\n", expectedConfig.kneuClientId)
		envFileContent += fmt.Sprintf("KNEU_CLIENT_SECRET=%s\n", expectedConfig.kneuClientSecret)

		envFileContent += fmt.Sprintf("JWT_SECRET_KEY=%s\n", expectedConfig.jwtSecretKey)
		envFileContent += fmt.Sprintf("APP_SECRET=%s\n", expectedConfig.appSecret)

		testEnvFilename := "TestLoadConfigFromFile.env"
		err := os.WriteFile(testEnvFilename, []byte(envFileContent), 0644)
		defer os.Remove(testEnvFilename)
		assert.NoErrorf(t, err, "got unexpected while write file %s error %s", testEnvFilename, err)

		config, err := loadConfig(testEnvFilename)

		assert.NoErrorf(t, err, "got unexpected error %s", err)
		assertConfig(t, expectedConfig, config)
		assert.Equalf(t, expectedConfig, config, "Expected for %v, actual: %v", expectedConfig, config)
	})

	t.Run("EmptyConfig", func(t *testing.T) {
		_ = os.Setenv("AUTHORIZER_PUBLIC_URL", "")
		_ = os.Setenv("LISTEN", "")
		_ = os.Setenv("KAFKA_HOST", "")
		_ = os.Setenv("KNEU_BASE_URI", "")
		_ = os.Setenv("KNEU_CLIENT_ID", "")
		_ = os.Setenv("KNEU_CLIENT_SECRET", "")
		_ = os.Setenv("JWT_SECRET_KEY", "")
		_ = os.Setenv("APP_SECRET", "")

		config, err := loadConfig("")

		assert.Error(t, err, "loadConfig() should exit with error, actual error is nil")

		assert.Emptyf(
			t, config.publicUrl,
			"Expected for empty config.publicUrl, actual %s", config.publicUrl,
		)

		assert.Emptyf(
			t, config.listenAddress,
			"Expected for empty config.listenAddress, actual %s", config.listenAddress,
		)
		assert.Emptyf(
			t, config.kneuBaseUri,
			"Expected for empty config.kneuBaseUri, actual %s", config.kneuBaseUri,
		)
		assert.Emptyf(
			t, config.kneuClientId,
			"Expected for empty config.kneuClientId, actual %d", config.kneuClientId,
		)

		assert.Emptyf(
			t, config.kneuClientSecret,
			"Expected for empty config.kneuClientSecret, actual %s", config.kneuClientSecret,
		)

		assert.Emptyf(
			t, config.appSecret,
			"Expected for empty config.kneuClientSecret, actual %s", config.kneuClientSecret,
		)
	})

	t.Run("EmptyPublicUrl", func(t *testing.T) {
		_ = os.Setenv("AUTHORIZER_PUBLIC_URL", "")
		_ = os.Setenv("LISTEN", expectedConfig.listenAddress)
		_ = os.Setenv("KAFKA_HOST", expectedConfig.kafkaHost)
		_ = os.Setenv("KNEU_BASE_URI", expectedConfig.kneuBaseUri)
		_ = os.Setenv("KNEU_CLIENT_ID", strconv.Itoa(int(expectedConfig.kneuClientId)))
		_ = os.Setenv("KNEU_CLIENT_SECRET", expectedConfig.kneuClientSecret)
		_ = os.Setenv("JWT_SECRET_KEY", string(expectedConfig.jwtSecretKey))

		config, err := loadConfig("")

		assert.Error(t, err, "loadConfig() should exit with error, actual error is nil")
		assert.Emptyf(
			t, config.publicUrl,
			"Expected for empty config.publicUrl, actual %s", config.publicUrl,
		)
	})

	t.Run("EmptyListen", func(t *testing.T) {
		_ = os.Setenv("AUTHORIZER_PUBLIC_URL", expectedConfig.publicUrl)
		_ = os.Setenv("LISTEN", "")
		_ = os.Setenv("KAFKA_HOST", expectedConfig.kafkaHost)
		_ = os.Setenv("KNEU_BASE_URI", expectedConfig.kneuBaseUri)
		_ = os.Setenv("KNEU_CLIENT_ID", strconv.Itoa(int(expectedConfig.kneuClientId)))
		_ = os.Setenv("KNEU_CLIENT_SECRET", expectedConfig.kneuClientSecret)
		_ = os.Setenv("JWT_SECRET_KEY", string(expectedConfig.jwtSecretKey))

		config, err := loadConfig("")

		assert.Error(t, err, "loadConfig() should exit with error, actual error is nil")
		assert.Emptyf(
			t, config.listenAddress,
			"Expected for empty config.listenAddress, actual %s", config.listenAddress,
		)
	})

	t.Run("EmptyKafka", func(t *testing.T) {
		_ = os.Setenv("AUTHORIZER_PUBLIC_URL", expectedConfig.publicUrl)
		_ = os.Setenv("LISTEN", expectedConfig.listenAddress)
		_ = os.Setenv("KAFKA_HOST", "")
		_ = os.Setenv("KNEU_BASE_URI", expectedConfig.kneuBaseUri)
		_ = os.Setenv("KNEU_CLIENT_ID", strconv.Itoa(int(expectedConfig.kneuClientId)))
		_ = os.Setenv("KNEU_CLIENT_SECRET", expectedConfig.kneuClientSecret)
		_ = os.Setenv("JWT_SECRET_KEY", string(expectedConfig.jwtSecretKey))

		config, err := loadConfig("")

		assert.Error(t, err, "loadConfig() should exit with error, actual error is nil")
		assert.Emptyf(
			t, config.kafkaHost,
			"Expected for empty config.kafkaHost, actual %s", config.kafkaHost,
		)
	})

	t.Run("EmptyKneuApiHost", func(t *testing.T) {
		_ = os.Setenv("AUTHORIZER_PUBLIC_URL", expectedConfig.publicUrl)
		_ = os.Setenv("LISTEN", expectedConfig.listenAddress)
		_ = os.Setenv("KAFKA_HOST", expectedConfig.kafkaHost)
		_ = os.Setenv("KNEU_BASE_URI", "")
		_ = os.Setenv("KNEU_CLIENT_ID", strconv.Itoa(int(expectedConfig.kneuClientId)))
		_ = os.Setenv("KNEU_CLIENT_SECRET", expectedConfig.kneuClientSecret)
		_ = os.Setenv("JWT_SECRET_KEY", string(expectedConfig.jwtSecretKey))

		_, err := loadConfig("")

		assert.NoError(t, err, "loadConfig() should not exit with error, actual error is nil")
	})

	t.Run("EmptyKneuClientId", func(t *testing.T) {
		_ = os.Setenv("AUTHORIZER_PUBLIC_URL", expectedConfig.publicUrl)
		_ = os.Setenv("LISTEN", expectedConfig.listenAddress)
		_ = os.Setenv("KAFKA_HOST", expectedConfig.kafkaHost)
		_ = os.Setenv("KNEU_BASE_URI", expectedConfig.kneuBaseUri)
		_ = os.Setenv("KNEU_CLIENT_ID", "")
		_ = os.Setenv("KNEU_CLIENT_SECRET", expectedConfig.kneuClientSecret)
		_ = os.Setenv("JWT_SECRET_KEY", string(expectedConfig.jwtSecretKey))

		config, err := loadConfig("")

		assert.Error(t, err, "loadConfig() should exit with error, actual error is nil")
		assert.Emptyf(
			t, config.kneuClientSecret,
			"Expected for empty config.listenAddress, actual %s", config.kneuClientSecret,
		)
	})

	t.Run("EmptyKneuClientSecret", func(t *testing.T) {
		_ = os.Setenv("AUTHORIZER_PUBLIC_URL", expectedConfig.publicUrl)
		_ = os.Setenv("LISTEN", expectedConfig.listenAddress)
		_ = os.Setenv("KAFKA_HOST", expectedConfig.kafkaHost)
		_ = os.Setenv("KNEU_BASE_URI", expectedConfig.kneuBaseUri)
		_ = os.Setenv("KNEU_CLIENT_ID", strconv.Itoa(int(expectedConfig.kneuClientId)))
		_ = os.Setenv("KNEU_CLIENT_SECRET", "")
		_ = os.Setenv("JWT_SECRET_KEY", string(expectedConfig.jwtSecretKey))

		config, err := loadConfig("")

		assert.Error(t, err, "loadConfig() should exit with error, actual error is nil")
		assert.Emptyf(
			t, config.kneuClientSecret,
			"Expected for empty config.kneuClientSecret, actual %s", config.kneuClientSecret,
		)
	})

	t.Run("EmptyJwtSecretKeyExpectDefault", func(t *testing.T) {
		_ = os.Setenv("AUTHORIZER_PUBLIC_URL", expectedConfig.publicUrl)
		_ = os.Setenv("LISTEN", expectedConfig.listenAddress)
		_ = os.Setenv("KAFKA_HOST", expectedConfig.kafkaHost)
		_ = os.Setenv("KNEU_BASE_URI", expectedConfig.kneuBaseUri)
		_ = os.Setenv("KNEU_CLIENT_ID", strconv.Itoa(int(expectedConfig.kneuClientId)))
		_ = os.Setenv("KNEU_CLIENT_SECRET", expectedConfig.kneuClientSecret)
		_ = os.Setenv("JWT_SECRET_KEY", "")

		config, err := loadConfig("")

		assert.NoErrorf(t, err, "loadConfig() should not exit with error, actual error: %v", err)
		assert.NotEmpty(t, config.jwtSecretKey)
	})

	t.Run("NotExistConfigFile", func(t *testing.T) {
		_ = os.Setenv("AUTHORIZER_PUBLIC_URL", "")
		_ = os.Setenv("LISTEN", "")
		_ = os.Setenv("KAFKA_HOST", "")
		_ = os.Setenv("KNEU_BASE_URI", "")
		_ = os.Setenv("KNEU_CLIENT_ID", "")
		_ = os.Setenv("KNEU_CLIENT_SECRET", "")
		_ = os.Setenv("JWT_SECRET_KEY", string(expectedConfig.jwtSecretKey))

		config, err := loadConfig("not-exists.env")

		assert.Error(t, err, "loadConfig() should exit with error, actual error is nil")
		assert.Equalf(
			t, "Error loading not-exists.env file: open not-exists.env: no such file or directory", err.Error(),
			"Expected for not exist file error, actual: %s", err.Error(),
		)
		assert.Emptyf(
			t, config.publicUrl,
			"Expected for empty config.publicUrl, actual %s", config.publicUrl,
		)

		assert.Emptyf(
			t, config.listenAddress,
			"Expected for empty config.listenAddress, actual %s", config.listenAddress,
		)
		assert.Emptyf(
			t, config.kneuBaseUri,
			"Expected for empty config.kneuBaseUri, actual %s", config.kneuBaseUri,
		)
		assert.Emptyf(
			t, config.kneuClientId,
			"Expected for empty config.kneuClientId, actual %d", config.kneuClientId,
		)

		assert.Emptyf(
			t, config.kneuClientSecret,
			"Expected for empty config.kneuClientSecret, actual %s", config.kneuClientSecret,
		)
	})
}

func assertConfig(t *testing.T, expected Config, actual Config) {
	assert.Equal(t, expected.publicUrl, actual.publicUrl)
	assert.Equal(t, expected.listenAddress, actual.listenAddress)
	assert.Equal(t, expected.kafkaHost, actual.kafkaHost)
	assert.Equal(t, expected.kneuBaseUri, actual.kneuBaseUri)
	assert.Equal(t, expected.kneuClientId, actual.kneuClientId)
	assert.Equal(t, expected.kneuClientSecret, actual.kneuClientSecret)
}
