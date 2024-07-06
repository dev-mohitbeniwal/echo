// api/util/http_util.go
package util

import (
	logger "github.com/dev-mohitbeniwal/echo/api/logging"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func RespondWithError(c *gin.Context, code int, message string, err error) {
	logger.Error(message,
		zap.Error(err),
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method))
	c.JSON(code, gin.H{"error": message})
}

func GetUserIDFromContext(c *gin.Context) (string, error) {
	userID, exists := c.Get("userID")
	if !exists {
		return "", nil
	}
	return userID.(string), nil
}
