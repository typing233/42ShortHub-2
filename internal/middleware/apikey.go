package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/42ShortHub/shortlink/internal/service"
)

func CombinedAuthMiddleware(secret string, apiKeySvc *service.APIKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
			key, err := apiKeySvc.Validate(apiKey)
			if err != nil {
				status := http.StatusUnauthorized
				msg := "invalid api key"
				switch err {
				case service.ErrAPIKeyExpired:
					msg = "api key has expired"
				case service.ErrAPIKeyInactive:
					msg = "api key is revoked"
				}
				c.AbortWithStatusJSON(status, gin.H{"code": 401, "message": msg})
				return
			}

			allowed, _ := apiKeySvc.CheckRateLimit(key)
			if !allowed {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"code": 429, "message": "api key rate limit exceeded",
				})
				return
			}

			allowed, _ = apiKeySvc.CheckQuota(key)
			if !allowed {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"code": 429, "message": "daily quota exceeded",
				})
				return
			}

			c.Set("user_id", key.UserID)
			c.Set("api_key_id", key.ID)
			c.Set("auth_method", "api_key")
			c.Next()
			return
		}

		tokenStr := ""
		if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			tokenStr = strings.TrimPrefix(auth, "Bearer ")
		}
		if tokenStr == "" {
			if cookie, err := c.Cookie("token"); err == nil {
				tokenStr = cookie
			}
		}

		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 401, "message": "missing authentication token",
			})
			return
		}

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 401, "message": "invalid or expired token",
			})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 401, "message": "invalid token claims",
			})
			return
		}

		userID := uint(claims["user_id"].(float64))
		username := claims["username"].(string)
		role := claims["role"].(string)

		c.Set("user_id", userID)
		c.Set("username", username)
		c.Set("role", role)
		c.Set("auth_method", "jwt")
		c.Next()
	}
}

func RequireAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code": 403, "message": "admin access required",
			})
			return
		}
		c.Next()
	}
}
