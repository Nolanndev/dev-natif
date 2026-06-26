package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *handler) pullImage(c *gin.Context) {
	var req pullImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failMsg(c, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.d.Deployments.PullImage(c.Request.Context(), req.Ref, req.AuthB64); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "pulled", "ref": req.Ref})
}

func (h *handler) buildImage(c *gin.Context) {
	var req buildImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failMsg(c, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.d.Deployments.BuildImage(c.Request.Context(), req.ContextDir, req.Dockerfile, req.Tag); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "built", "tag": req.Tag})
}
