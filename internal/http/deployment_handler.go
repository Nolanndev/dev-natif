package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *handler) createDeployment(c *gin.Context) {
	var req createDeploymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failMsg(c, http.StatusBadRequest, "invalid JSON body")
		return
	}
	dep, err := h.d.Deployments.CreateDeployment(
		c.Request.Context(), c.Param("id"), req.Name, req.ServerID, req.overridesToDomain(),
	)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, dep)
}

func (h *handler) listDeployments(c *gin.Context) {
	ds, err := h.d.Deployments.ListDeployments(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, ds)
}

func (h *handler) getDeployment(c *gin.Context) {
	dep, err := h.d.Deployments.GetDeployment(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dep)
}

func (h *handler) deleteDeployment(c *gin.Context) {
	if err := h.d.Deployments.DeleteDeployment(c.Request.Context(), c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *handler) upDeployment(c *gin.Context) {
	dep, err := h.d.Deployments.Up(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dep)
}

func (h *handler) downDeployment(c *gin.Context) {
	if err := h.d.Deployments.Down(c.Request.Context(), c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "down"})
}

func (h *handler) statusDeployment(c *gin.Context) {
	status, containers, err := h.d.Deployments.Status(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, statusResponse{Status: status, Containers: containers})
}
