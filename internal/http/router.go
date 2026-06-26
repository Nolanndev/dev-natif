package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Nolanndev/dev-natif/internal/auth"
	"github.com/Nolanndev/dev-natif/internal/domain"
	"github.com/Nolanndev/dev-natif/internal/service"
)

// Deps holds everything the HTTP layer needs. main.go constructs it.
type Deps struct {
	Logger      *slog.Logger
	Auth        *auth.Authenticator
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

	// Embedded web console (single-page app), served same-origin at /app.
	r.StaticFS("/app", http.FS(webRoot()))
	r.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, "/app/") })

	// Operational endpoints (no auth).
	r.GET("/healthz", h.healthz)
	r.GET("/readyz", h.readyz)

	// Login is public; everything else under /api/v1 requires a valid token.
	r.POST("/api/v1/auth/login", h.login)

	v1 := r.Group("/api/v1")
	v1.Use(authBearer(d.Auth))

	// Auth (token lifecycle)
	v1.POST("/auth/refresh", h.refresh)
	v1.GET("/auth/me", h.me)

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
	v1.GET("/projects/:id/deployments", h.listProjectDeployments) // deployment history
	v1.GET("/projects/:id/events", h.listProjectEvents)
	v1.GET("/deployments", h.listDeployments)
	v1.GET("/deployments/:id", h.getDeployment)
	v1.DELETE("/deployments/:id", h.deleteDeployment)
	v1.POST("/deployments/:id/up", h.upDeployment)
	v1.POST("/deployments/:id/down", h.downDeployment)
	v1.GET("/deployments/:id/status", h.statusDeployment)
	v1.GET("/deployments/:id/events", h.listDeploymentEvents)
	v1.GET("/deployments/:id/containers/:cid/logs", h.containerLogs)

	// Events (global activity / errors feed)
	v1.GET("/events", h.listEvents)

	// Images
	v1.GET("/images", h.listImages)
	v1.POST("/images/pull", h.pullImage)
	v1.POST("/images/build", h.buildImage)

	// Servers
	v1.GET("/servers", h.listServers)
	v1.GET("/servers/:id", h.getServer)

	return r
}
