package main

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

func loadEnv() {
	paths := []string{
		".env",
		"backend-go/.env",
		"../backend-go/.env",
	}

	for _, path := range paths {
		if err := godotenv.Load(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			appLogger.Warn("env_load_failed", "path", path, "error", err)
		}
	}
}
