package main

import (
	"fundtracking/internal/handler"
	"fundtracking/internal/repository"
	"fundtracking/internal/service"
	"log"
	"net/http"
)

func main() {
	runtimeRepo := repository.NewRuntimeRepository()
	healthService := service.NewHealthService(runtimeRepo)
	healthHandler := handler.NewHealthHandler(healthService)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/version", healthHandler.APIVersion)
	log.Println("starting server on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
