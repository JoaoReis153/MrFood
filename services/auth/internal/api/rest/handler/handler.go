package handler

import (
	"net/http"

	"MrFood/services/auth/internal/app"

	"github.com/gin-gonic/gin"

	"MrFood/services/auth/pkg"
)

type Handler struct {
	app *app.App
}

func New(app *app.App) *Handler {
	return &Handler{app: app}
}

func (h *Handler) Health(c *gin.Context) {
	response := pkg.SuccessResponse[struct{}]{
		Data:    struct{}{},
		Message: "Service auth is healthy",
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handler) Ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "pong pang"})
}
