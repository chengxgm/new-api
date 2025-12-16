package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

// SetChannelMgrRouter registers a lightweight admin-only channel CRUD manager API.
// Route prefix: /api/channel/mgr
func SetChannelMgrRouter(apiRouter *gin.RouterGroup) {
	mgr := apiRouter.Group("/channel/mgr")
	mgr.Use(middleware.AdminAuth())
	{
		mgr.GET("/meta", controller.ChannelMgrMeta)
		mgr.GET("/list", controller.ChannelMgrList)
		mgr.POST("/create", controller.ChannelMgrCreate)
		mgr.POST("/update", controller.ChannelMgrUpdate)
		mgr.POST("/delete", controller.ChannelMgrDelete)
		mgr.POST("/copy", controller.ChannelMgrCopy)
		mgr.POST("/batch_update", controller.ChannelMgrBatchUpdate)
		mgr.POST("/batch_copy", controller.ChannelMgrBatchCopy)
	}
}
