package router

import (
	"github.com/gin-gonic/gin"
	"MrFood/services/auth/internal/app"
	"MrFood/services/auth/internal/api/rest/handler"
)

func New(app *app.App) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	h := handler.New(app)

	api := r.Group("/api/v1")
	{
		api.GET("/health", h.Health)
		api.GET("/ping", h.Ping)
	}

	return r
}
