package httpapi

import (
	"net/http"
	"strings"

	"ai-interview-platform/internal/auth"

	"github.com/gin-gonic/gin"
)

const (
	currentUserIDKey = "current_user_id"
	currentRoleKey   = "current_user_role"
)

func (h apiHandler) register(c *gin.Context) {
	var req auth.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	pair, user, err := h.deps.AuthService.Register(c.Request.Context(), req)
	if err != nil {
		writeGinError(c, http.StatusBadRequest, "register_failed", err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"schema_version": "auth.token_pair.v1",
		"user":           user,
		"tokens":         pair,
	})
}

func (h apiHandler) login(c *gin.Context) {
	var req auth.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	pair, user, err := h.deps.AuthService.Login(c.Request.Context(), req, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		writeGinError(c, http.StatusUnauthorized, "login_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"schema_version": "auth.token_pair.v1",
		"user":           user,
		"tokens":         pair,
	})
}

func (h apiHandler) refreshToken(c *gin.Context) {
	var req auth.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	pair, user, err := h.deps.AuthService.Refresh(c.Request.Context(), req, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		writeGinError(c, http.StatusUnauthorized, "refresh_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"schema_version": "auth.token_pair.v1",
		"user":           user,
		"tokens":         pair,
	})
}

func (h apiHandler) logout(c *gin.Context) {
	var req auth.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeGinError(c, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := h.deps.AuthService.Logout(c.Request.Context(), req); err != nil {
		writeGinError(c, http.StatusBadRequest, "logout_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"schema_version": "auth.logout.v1", "ok": true})
}

func (h apiHandler) me(c *gin.Context) {
	user, err := h.deps.AuthService.Me(c.Request.Context(), currentUserID(c))
	if err != nil {
		writeGinError(c, http.StatusUnauthorized, "auth_user_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"schema_version": "auth.me.v1", "user": user})
}

func (h apiHandler) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.deps.Config.AuthDisabled {
			c.Set(currentUserIDKey, "root")
			c.Set(currentRoleKey, "root")
			c.Next()
			return
		}
		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" {
			writeGinError(c, http.StatusUnauthorized, "missing_token", "Authorization Bearer token is required")
			c.Abort()
			return
		}
		claims, err := h.deps.AuthService.AuthenticateAccessToken(token)
		if err != nil || claims.Type != "access" || claims.Sub == "" {
			writeGinError(c, http.StatusUnauthorized, "invalid_token", "access token is invalid or expired")
			c.Abort()
			return
		}
		user, err := h.deps.AuthService.Me(c.Request.Context(), claims.Sub)
		if err != nil {
			writeGinError(c, http.StatusUnauthorized, "auth_user_failed", "current user is disabled or not found")
			c.Abort()
			return
		}
		c.Set(currentUserIDKey, user.UserID)
		c.Set(currentRoleKey, user.Role)
		c.Next()
	}
}

func (h apiHandler) requireRoot() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentRole(c) != "root" {
			writeGinError(c, http.StatusForbidden, "root_required", "root role is required")
			c.Abort()
			return
		}
		c.Next()
	}
}

func currentUserID(c *gin.Context) string {
	value, _ := c.Get(currentUserIDKey)
	userID, _ := value.(string)
	return userID
}

func currentRole(c *gin.Context) string {
	value, _ := c.Get(currentRoleKey)
	role, _ := value.(string)
	return role
}

func canAccessUser(c *gin.Context, ownerID string) bool {
	if currentRole(c) == "root" {
		return true
	}
	return currentUserID(c) != "" && currentUserID(c) == ownerID
}

func bearerToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	prefix := "Bearer "
	if !strings.HasPrefix(value, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(value, prefix))
}
