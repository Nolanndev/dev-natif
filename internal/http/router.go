package httpapi

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/actigraph/dev-natif/internal/domain"
	"github.com/actigraph/dev-natif/internal/service"
)

// Deps holds everything the HTTP layer needs. main.go constructs it.
type Deps struct {
	Logger      *slog.Logger
	APIKey      string
	Projects    *service.ProjectService
	Deployments *service.DeploymentService
	Servers     domain.ServerRepository
	Engine      domain.DockerEngine
}

type handler struct {
	d Deps
}

// NewRouter builds the Gin engine with all middleware and routes wired.
func NewRouter(d Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(requestID(), requestLogger(d.Logger), recovery(d.Logger))

	h := &handler{d: d}

	// Operational endpoints (no auth).
	r.GET("/healthz", h.healthz)
	r.GET("/readyz", h.readyz)

	v1 := r.Group("/api/v1")
	v1.Use(apiKeyAuth(d.APIKey))

	// Projects
	v1.POST("/projects", h.createProject)
	v1.GET("/projects", h.listProjects)
	v1.GET("/projects/:id", h.getProject)
	v1.PUT("/projects/:id", h.updateProject)
	v1.DELETE("/projects/:id", h.deleteProject)

	// Services (nested under a project)
	v1.POST("/projects/:id/services", h.addService)
	v1.GET("/projects/:id/services", h.listServices)
	v1.PUT("/projects/:id/services/:sid", h.updateService)
	v1.DELETE("/projects/:id/services/:sid", h.deleteService)

	// Volumes (nested under a project)
	v1.POST("/projects/:id/volumes", h.addVolume)
	v1.GET("/projects/:id/volumes", h.listVolumes)
	v1.DELETE("/projects/:id/volumes/:vid", h.deleteVolume)

	// Deployments
	v1.POST("/projects/:id/deployments", h.createDeployment)
	v1.GET("/deployments", h.listDeployments)
	v1.GET("/deployments/:id", h.getDeployment)
	v1.DELETE("/deployments/:id", h.deleteDeployment)
	v1.POST("/deployments/:id/up", h.upDeployment)
	v1.POST("/deployments/:id/down", h.downDeployment)
	v1.GET("/deployments/:id/status", h.statusDeployment)

	// Images
	v1.POST("/images/pull", h.pullImage)
	v1.POST("/images/build", h.buildImage)

	// Servers
	v1.GET("/servers", h.listServers)
	v1.GET("/servers/:id", h.getServer)

	return r
}
