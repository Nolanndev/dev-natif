package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *handler) listServers(c *gin.Context) {
	servers, err := h.d.Servers.ListServers(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, servers)
}

func (h *handler) getServer(c *gin.Context) {
	srv, err := h.d.Servers.GetServer(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, srv)
}
