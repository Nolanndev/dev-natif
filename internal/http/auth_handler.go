package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Nolanndev/dev-natif/internal/auth"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// login authenticates credentials and returns a signed, expiring token.
func (h *handler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failMsg(c, http.StatusBadRequest, "invalid JSON body")
		return
	}
	tok, err := h.d.Auth.Login(req.Username, req.Password)
	if err != nil {
		failMsg(c, http.StatusUnauthorized, "invalid credentials")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":      tok.Value,
		"token_type": "Bearer",
		"expires_at": tok.ExpiresAt,
		"username":   tok.Subject,
	})
}

// refresh issues a new token from the current valid one (renewal).
func (h *handler) refresh(c *gin.Context) {
	tokenStr := bearerToken(c)
	tok, err := h.d.Auth.Refresh(tokenStr)
	if err != nil {
		failMsg(c, http.StatusUnauthorized, "invalid or expired token")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":      tok.Value,
		"token_type": "Bearer",
		"expires_at": tok.ExpiresAt,
		"username":   tok.Subject,
	})
}

// me returns the identity tied to the current token.
func (h *handler) me(c *gin.Context) {
	sub, exp, err := h.d.Auth.Parse(bearerToken(c))
	if err != nil {
		failMsg(c, http.StatusUnauthorized, "invalid or expired token")
		return
	}
	c.JSON(http.StatusOK, gin.H{"username": sub, "expires_at": exp})
}

// bearerToken extracts the token from the Authorization header.
func bearerToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if v, ok := strings.CutPrefix(h, "Bearer "); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// authBearer enforces a valid bearer token when auth is enabled.
func authBearer(a *auth.Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		if a == nil || !a.Enabled() {
			c.Next()
			return
		}
		sub, _, err := a.Parse(bearerToken(c))
		if err != nil {
			status := http.StatusUnauthorized
			if errors.Is(err, auth.ErrInvalidToken) {
				status = http.StatusUnauthorized
			}
			c.AbortWithStatusJSON(status, errorResponse{Error: "unauthorized"})
			return
		}
		c.Set("user", sub)
		c.Next()
	}
}
