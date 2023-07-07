//go:build !test

package main

import (
	"net/http"
	"os"
)

func main() {
	os.Exit(handleExitError(os.Stderr, runApp(os.Stdout, http.ListenAndServe)))
}
