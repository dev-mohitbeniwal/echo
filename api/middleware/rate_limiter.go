// api/middleware/rate_limiter.go

package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dev-mohitbeniwal/echo/api/db"
	logger "github.com/dev-mohitbeniwal/echo/api/logging"
)

func RateLimiter(limit int, per time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP() // Or use a user identifier
		allowed, err := db.RateLimit(c, key, limit, per)
		if err != nil {
			logger.Error("Rate limiting failed", zap.Error(err), zap.String("ip", key))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Rate limiting failed"})
			c.Abort()
			return
		}

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Duration", per.String())

		if !allowed {
			logger.Warn("Rate limit exceeded",
				zap.String("ip", key),
				zap.Int("limit", limit),
				zap.Duration("per", per))
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded"})
			c.Abort()
			return
		}

		logger.Info("Request allowed",
			zap.String("ip", key),
			zap.Int("limit", limit),
			zap.Duration("per", per))
		c.Next()
	}
}
