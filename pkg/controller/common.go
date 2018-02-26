package controller

import (
	"context"
	"strings"

	"github.com/codefresh-io/hermes/pkg/model"

	"github.com/gin-gonic/gin"
)

// ErrorResult returned by controllers
type ErrorResult struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

type contextKey string

func getParam(c *gin.Context, name string) string {
	v := c.Param(name)
	return strings.Replace(v, "_slash_", "/", -1)
}

func getContext(c *gin.Context) context.Context {
	account := c.Param("account")
	return context.WithValue(context.Background(), model.ContextKeyAccount, account)
}
