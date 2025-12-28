package handler

import (
	"GoChat/internel/model/dto"
	"GoChat/internel/service"
	"GoChat/pkg/util"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GroupHandler 群组处理
type GroupHandler struct {
	groupService service.IGroupService
}

func NewGroupHandler(gs service.IGroupService) *GroupHandler {
	return &GroupHandler{
		groupService: gs,
	}
}

func (gh *GroupHandler) CreateGroup(c *gin.Context) {
	var req dto.CreateGroupReq
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusServiceUnavailable, util.NewResMsg("0", "服务器繁忙，请稍后重试", nil))
		return
	}

	var ctx = c.Request.Context()
	var resp *dto.CreateGroupResp
	var err error
	if resp, err = gh.groupService.NewGroup(ctx, &req); err != nil {
		return
	}
	c.JSON(http.StatusOK, util.NewResMsg("1", "创建群组成功", resp))
}
