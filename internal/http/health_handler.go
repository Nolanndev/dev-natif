package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// healthz reports process liveness.
func (h *handler) healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// readyz reports readiness, including Docker Engine reachability.
func (h *handler) readyz(c *gin.Context) {
	engine := "ok"
	if err := h.d.Engine.Ping(c.Request.Context()); err != nil {
		engine = "unreachable"
	}
	status := http.StatusOK
	if engine != "ok" {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, gin.H{"status": "ready", "docker_engine": engine})
}
