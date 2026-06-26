package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Nolanndev/dev-natif/internal/domain"
)

func (h *handler) createProject(c *gin.Context) {
	var req createProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failMsg(c, http.StatusBadRequest, "invalid JSON body")
		return
	}
	p := &domain.Project{Name: req.Name, Description: req.Description}
	if err := h.d.Projects.CreateProject(c.Request.Context(), p); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (h *handler) listProjects(c *gin.Context) {
	ps, err := h.d.Projects.ListProjects(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, ps)
}

func (h *handler) getProject(c *gin.Context) {
	p, err := h.d.Projects.GetProject(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *handler) updateProject(c *gin.Context) {
	var req updateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failMsg(c, http.StatusBadRequest, "invalid JSON body")
		return
	}
	p := &domain.Project{ID: c.Param("id"), Name: req.Name, Description: req.Description}
	if err := h.d.Projects.UpdateProject(c.Request.Context(), p); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *handler) deleteProject(c *gin.Context) {
	id := c.Param("id")
	// Tear down any running containers of this project before deleting its
	// records, so nothing is orphaned on the engine. Best effort.
	_ = h.d.Deployments.DownProject(c.Request.Context(), id)
	if err := h.d.Projects.DeleteProject(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- Services --------------------------------------------------------------

func (h *handler) addService(c *gin.Context) {
	var req serviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failMsg(c, http.StatusBadRequest, "invalid JSON body")
		return
	}
	svc := req.toDomain()
	if err := h.d.Projects.AddService(c.Request.Context(), c.Param("id"), svc); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, svc)
}

func (h *handler) listServices(c *gin.Context) {
	ss, err := h.d.Projects.ListServices(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, ss)
}

func (h *handler) updateService(c *gin.Context) {
	var req serviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failMsg(c, http.StatusBadRequest, "invalid JSON body")
		return
	}
	svc := req.toDomain()
	svc.ID = c.Param("sid")
	svc.ProjectID = c.Param("id")
	if err := h.d.Projects.UpdateService(c.Request.Context(), svc); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, svc)
}

func (h *handler) deleteService(c *gin.Context) {
	if err := h.d.Projects.DeleteService(c.Request.Context(), c.Param("sid")); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- Volumes ---------------------------------------------------------------

func (h *handler) addVolume(c *gin.Context) {
	var req volumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failMsg(c, http.StatusBadRequest, "invalid JSON body")
		return
	}
	v := &domain.Volume{Name: req.Name, Driver: req.Driver}
	if err := h.d.Projects.AddVolume(c.Request.Context(), c.Param("id"), v); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, v)
}

func (h *handler) listVolumes(c *gin.Context) {
	vs, err := h.d.Projects.ListVolumes(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, vs)
}

func (h *handler) deleteVolume(c *gin.Context) {
	if err := h.d.Projects.DeleteVolume(c.Request.Context(), c.Param("vid")); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
