package helper_util

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetPaginationParams(c *gin.Context) (limit int, offset int, err error) {
	limit, err = strconv.Atoi(c.DefaultQuery("limit", "10"))
	if err != nil {
		return 0, 0, err
	}
	offset, err = strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil {
		return 0, 0, err
	}
	return limit, offset, nil
}
