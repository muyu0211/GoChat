package router

import (
	"github.com/gin-gonic/gin"
)

func RegisterGroupRouter(r *gin.Engine) {
	groupApi := r.Group("/group")
	//
	//dbs := db.GetDBS()
	//tx := service.NewTxManager(dbs.Master)
	//groupService := service.NewGroupService()
	//groupHandler := handler.NewGroupHandler()
	{
		groupApi.POST("/create")
	}
}
