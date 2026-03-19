package router

import (
	"MrFood/services/restaurant/internal/api/rest/handler"
	"MrFood/services/restaurant/internal/app"

	"github.com/gin-gonic/gin"
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
