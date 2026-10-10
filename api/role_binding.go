package api

import (
	"strconv"

	middleware "watchAlert/internal/middleware"
	"watchAlert/internal/services"
	"watchAlert/internal/types"
	"watchAlert/pkg/response"

	"github.com/gin-gonic/gin"
)

type roleBindingController struct{}

var RoleBindingController = new(roleBindingController)

// API 注册角色绑定管理接口：
// POST   /api/watchalert/role-bindings
// GET    /api/watchalert/role-bindings
// DELETE /api/watchalert/role-bindings/:eiam_uid
func (roleBindingController roleBindingController) API(gin *gin.RouterGroup) {
	g := gin.Group("role-bindings")
	g.Use(middleware.Auth())
	{
		g.POST("", roleBindingController.Create)
		g.GET("", roleBindingController.List)
		g.DELETE(":eiam_uid", roleBindingController.Delete)
	}
}

func (roleBindingController roleBindingController) Create(ctx *gin.Context) {
	r := new(types.RequestRoleBindingCreate)
	if err := ctx.ShouldBindJSON(r); err != nil {
		response.Envelope(ctx, nil, response.NewValidationError("请求体解析失败"))
		return
	}
	data, err := services.RoleBindingService.Create(r)
	response.Envelope(ctx, data, err)
}

func (roleBindingController roleBindingController) List(ctx *gin.Context) {
	r := new(types.RequestRoleBindingList)
	if err := ctx.ShouldBindQuery(r); err != nil {
		response.Envelope(ctx, nil, response.NewValidationError("查询参数解析失败"))
		return
	}
	data, err := services.RoleBindingService.List(r)
	response.Envelope(ctx, data, err)
}

func (roleBindingController roleBindingController) Delete(ctx *gin.Context) {
	eiamUid, err := strconv.ParseInt(ctx.Param("eiam_uid"), 10, 64)
	if err != nil {
		response.Envelope(ctx, nil, response.NewValidationError("eiam_uid 校验失败"))
		return
	}
	data, err := services.RoleBindingService.Delete(eiamUid)
	response.Envelope(ctx, data, err)
}
