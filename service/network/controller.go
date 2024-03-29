package network

import "github.com/gin-gonic/gin"

type Controller struct{}

func (controller *Controller) GetPoolStats(c *gin.Context) {
	poolStats, err := GetPoolStats()
	if err != nil {
		_ = c.Error(ErrorUnableToRetrieveStats).SetType(gin.ErrorTypePublic)
		return
	}

	c.JSON(200, poolStats)
}
