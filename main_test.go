package main

import (
	"bytes"
	"errors"
	"github.com/stretchr/testify/assert"
	"net/http"
	"os"
	"strconv"
	"testing"
)

func TestRunApp(t *testing.T) {
	t.Run("Run with mock config", func(t *testing.T) {
		_ = os.Setenv("PUBLIC_URL", expectedConfig.publicUrl)
		_ = os.Setenv("LISTEN", expectedConfig.listenAddress)
		_ = os.Setenv("KAFKA_HOST", expectedConfig.kafkaHost)
		_ = os.Setenv("KNEU_CLIENT_ID", strconv.Itoa(int(expectedConfig.kneuClientId)))
		_ = os.Setenv("KNEU_CLIENT_SECRET", expectedConfig.kneuClientSecret)

		var out bytes.Buffer

		actualListen := ""
		listenAndServe := func(listen string, _ http.Handler) error {
			actualListen = listen
			return nil
		}

		err := runApp(&out, listenAndServe)

		assert.NoError(t, err, "Expected for TooManyError, got %s", err)
		assert.Equal(t, expectedConfig.listenAddress, actualListen)
	})

	t.Run("Run with wrong env file", func(t *testing.T) {
		previousWd, err := os.Getwd()
		assert.NoErrorf(t, err, "Failed to get working dir: %s", err)
		tmpDir := os.TempDir() + "/secondary-db-watcher-run-dir"
		tmpEnvFilepath := tmpDir + "/.env"

		defer func() {
			_ = os.Chdir(previousWd)
			_ = os.Remove(tmpEnvFilepath)
			_ = os.Remove(tmpDir)
		}()

		if _, err := os.Stat(tmpDir); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(tmpDir, os.ModePerm)
			assert.NoErrorf(t, err, "Failed to create tmp dir %s: %s", tmpDir, err)
		}
		if _, err := os.Stat(tmpEnvFilepath); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(tmpEnvFilepath, os.ModePerm)
			assert.NoErrorf(t, err, "Failed to create tmp  %s/.env: %s", tmpDir, err)
		}

		err = os.Chdir(tmpDir)
		assert.NoErrorf(t, err, "Failed to change working dir: %s", err)

		listenAndServe := func(string, http.Handler) error {
			return nil
		}

		var out bytes.Buffer
		err = runApp(&out, listenAndServe)

		assert.Error(t, err, "Expected for error")
		assert.Containsf(
			t, err.Error(), "Error loading .env file",
			"Expected for Load config error, got: %s", err,
		)
	})
}

func TestHandleExitError(t *testing.T) {
	t.Run("Handle exit error", func(t *testing.T) {
		var actualExitCode int
		var out bytes.Buffer

		testCases := map[error]int{
			errors.New("dummy error"): ExitCodeMainError,
			nil:                       0,
		}

		for err, expectedCode := range testCases {
			out.Reset()
			actualExitCode = handleExitError(&out, err)

			assert.Equalf(
				t, expectedCode, actualExitCode,
				"Expect handleExitError(%v) = %d, actual: %d",
				err, expectedCode, actualExitCode,
			)
			if err == nil {
				assert.Empty(t, out.String(), "Error is not empty")
			} else {
				assert.Contains(t, out.String(), err.Error(), "error output hasn't error description")
			}
		}
	})
}
