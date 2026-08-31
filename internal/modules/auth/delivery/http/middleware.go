package http

import (
	jwtinfra "backend/internal/modules/auth/infrastructure/jwt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func RequireAuth(jwt *jwtinfra.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "bearer token required"})
			return
		}
		claims, err := jwt.Parse(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		c.Set("user_id", claims["sub"])
		c.Set("role", claims["role"])
		c.Next()
	}
}

// RequireWebSocketAuth accepts the normal Bearer header. Browser clients can
// send protocols ["bearer", "JWT"] so the credential travels in a header.
func RequireWebSocketAuth(jwt *jwtinfra.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ""
		header := c.GetHeader("Authorization")
		if strings.HasPrefix(header, "Bearer ") {
			token = strings.TrimPrefix(header, "Bearer ")
		} else {
			protocols := strings.Split(c.GetHeader("Sec-WebSocket-Protocol"), ",")
			if len(protocols) >= 2 && strings.TrimSpace(protocols[0]) == "bearer" {
				token = strings.TrimSpace(protocols[1])
			}
		}
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "access token required"})
			return
		}
		claims, err := jwt.Parse(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		c.Set("user_id", claims["sub"])
		c.Set("role", claims["role"])
		c.Next()
	}
}

func RequireRoles(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "role is missing from token"})
			return
		}
		roleName, valid := role.(string)
		if !valid {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid role in token"})
			return
		}
		if _, ok := allowed[roleName]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
		c.Next()
	}
}
