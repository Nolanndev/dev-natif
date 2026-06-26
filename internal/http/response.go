package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/actigraph/dev-natif/internal/domain"
)

// errorResponse is the uniform error envelope.
type errorResponse struct {
	Error string `json:"error"`
}

// statusFor maps domain sentinel errors to HTTP status codes.
func statusFor(err error) int {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrValidation):
		return http.StatusBadRequest
	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, domain.ErrDependencyCyc):
		return http.StatusUnprocessableEntity
	case errors.Is(err, domain.ErrDockerEngine):
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

// fail writes a JSON error with the status mapped from the error.
func fail(c *gin.Context, err error) {
	c.JSON(statusFor(err), errorResponse{Error: err.Error()})
}

// failMsg writes a JSON error with an explicit status and message.
func failMsg(c *gin.Context, status int, msg string) {
	c.JSON(status, errorResponse{Error: msg})
}
