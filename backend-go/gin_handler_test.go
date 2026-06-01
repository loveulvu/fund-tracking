package main

import (
	"io"
	"net/http/httptest"
	"strings"

	"github.com/gin-gonic/gin"
)

func performGinRequest(method string, path string, body io.Reader, handlers ...gin.HandlerFunc) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	routePath, _, _ := strings.Cut(path, "?")
	router.Handle(method, routePath, handlers...)

	request := httptest.NewRequest(method, path, body)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
