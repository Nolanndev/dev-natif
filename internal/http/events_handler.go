package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// listProjectDeployments returns a project's deployment history.
func (h *handler) listProjectDeployments(c *gin.Context) {
	ds, err := h.d.Deployments.ListProjectDeployments(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, ds)
}

// listProjectEvents returns recent events for a project.
func (h *handler) listProjectEvents(c *gin.Context) {
	evs, err := h.d.Deployments.ListProjectEvents(c.Request.Context(), c.Param("id"), queryLimit(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, evs)
}

// listDeploymentEvents returns recent events for one deployment.
func (h *handler) listDeploymentEvents(c *gin.Context) {
	evs, err := h.d.Deployments.ListDeploymentEvents(c.Request.Context(), c.Param("id"), queryLimit(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, evs)
}

// listEvents returns the most recent events across the system (activity feed).
func (h *handler) listEvents(c *gin.Context) {
	evs, err := h.d.Deployments.ListRecentEvents(c.Request.Context(), queryLimit(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, evs)
}

// containerLogs returns the recent logs of one container of a deployment.
func (h *handler) containerLogs(c *gin.Context) {
	tail := 200
	if v := c.Query("tail"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			tail = n
		}
	}
	logs, err := h.d.Deployments.ContainerLogs(c.Request.Context(), c.Param("cid"), tail)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

func queryLimit(c *gin.Context) int {
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 100
}
