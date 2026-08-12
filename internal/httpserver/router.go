package httpserver

import (
	"log/slog"
	"net/http"

	"cloudsentinel/internal/asset"
	"cloudsentinel/internal/auth"
	"cloudsentinel/internal/health"
	"cloudsentinel/internal/incident"
	appmiddleware "cloudsentinel/internal/middleware"
	"cloudsentinel/internal/probe"
	"github.com/gin-gonic/gin"
)

type Modules struct {
	AuthHandler       *auth.Handler
	AuthMiddleware    gin.HandlerFunc
	AssetHandler      *asset.Handler
	ProbeHandler      *probe.Handler
	MetricsMiddleware gin.HandlerFunc
	MetricsHandler    http.Handler
	IncidentHandler   *incident.Handler
	MachineAuth       gin.HandlerFunc
}

func NewRouter(log *slog.Logger, healthHandler *health.Handler, modules ...Modules) *gin.Engine {
	router := gin.New()
	router.Use(appmiddleware.RequestID())
	router.Use(appmiddleware.AccessLog(log))
	router.Use(appmiddleware.Recovery(log))
	if len(modules) > 0 && modules[0].MetricsMiddleware != nil {
		router.Use(modules[0].MetricsMiddleware)
	}
	router.GET("/healthz", healthHandler.Health)
	router.GET("/readyz", healthHandler.Ready)
	if len(modules) > 0 && modules[0].MetricsHandler != nil {
		router.GET("/metrics", gin.WrapH(modules[0].MetricsHandler))
	}
	if len(modules) > 0 && modules[0].IncidentHandler != nil && modules[0].MachineAuth != nil {
		router.POST("/api/v1/alerts/webhook", modules[0].MachineAuth, modules[0].IncidentHandler.Webhook)
	}
	if len(modules) > 0 && modules[0].AuthHandler != nil {
		api := router.Group("/api/v1")
		api.POST("/auth/login", modules[0].AuthHandler.Login)
		if modules[0].AuthMiddleware != nil {
			protected := api.Group("")
			protected.Use(modules[0].AuthMiddleware)
			protected.GET("/users/me", modules[0].AuthHandler.Me)
			if modules[0].AssetHandler != nil {
				assets := protected.Group("")
				assets.POST("/hosts", modules[0].AssetHandler.CreateHost)
				assets.GET("/hosts", modules[0].AssetHandler.ListHosts)
				assets.GET("/hosts/:id", modules[0].AssetHandler.GetHost)
				assets.PUT("/hosts/:id", modules[0].AssetHandler.UpdateHost)
				assets.DELETE("/hosts/:id", modules[0].AssetHandler.DisableHost)
				assets.POST("/services", modules[0].AssetHandler.CreateService)
				assets.GET("/services", modules[0].AssetHandler.ListServices)
				assets.GET("/services/:id", modules[0].AssetHandler.GetService)
				assets.PUT("/services/:id", modules[0].AssetHandler.UpdateService)
				assets.DELETE("/services/:id", modules[0].AssetHandler.DisableService)
			}
			if modules[0].ProbeHandler != nil {
				protected.POST("/probe-tasks", modules[0].ProbeHandler.Create)
				protected.GET("/probe-tasks", modules[0].ProbeHandler.List)
				protected.GET("/probe-tasks/:id", modules[0].ProbeHandler.Get)
				protected.PUT("/probe-tasks/:id", modules[0].ProbeHandler.Update)
				protected.DELETE("/probe-tasks/:id", modules[0].ProbeHandler.Disable)
				protected.GET("/probe-results", modules[0].ProbeHandler.ListResults)
				protected.GET("/probe-results/:id", modules[0].ProbeHandler.GetResult)
			}
			if modules[0].IncidentHandler != nil {
				protected.GET("/incidents", modules[0].IncidentHandler.List)
				protected.GET("/incidents/:id", modules[0].IncidentHandler.Get)
				protected.POST("/incidents/:id/acknowledge", modules[0].IncidentHandler.Acknowledge)
				protected.POST("/incidents/:id/process", modules[0].IncidentHandler.Process)
				protected.POST("/incidents/:id/close", modules[0].IncidentHandler.Close)
			}
		}
	}
	return router
}
