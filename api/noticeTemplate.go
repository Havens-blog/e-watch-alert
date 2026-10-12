package api

import (
	"watchAlert/internal/middleware"
	"watchAlert/internal/services"
	"watchAlert/internal/types"

	"github.com/gin-gonic/gin"
)

type noticeTemplateController struct{}

var NoticeTemplateController = new(noticeTemplateController)

/*
通知模版 API
/api/w8t/noticeTemplate
*/
func (noticeTemplateController noticeTemplateController) API(gin *gin.RouterGroup) {
	a := gin.Group("noticeTemplate")
	a.Use(
		middleware.Auth(),
		middleware.Permission(),
		middleware.ParseTenant(),
		middleware.AuditingLog(),
	)
	{
		a.POST("noticeTemplateCreate", noticeTemplateController.Create)
		a.POST("noticeTemplateUpdate", noticeTemplateController.Update)
		a.POST("noticeTemplateDelete", noticeTemplateController.Delete)
	}
	b := gin.Group("noticeTemplate")
	b.Use(
		middleware.Auth(),
		middleware.Permission(),
		middleware.ParseTenant(),
	)
	{
		b.GET("noticeTemplateList", noticeTemplateController.List)
		b.GET("noticeTemplateGet", noticeTemplateController.Get)
	}
}

func (noticeTemplateController noticeTemplateController) Create(ctx *gin.Context) {
	r := new(types.RequestNoticeTemplateCreate)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		// 会话化后由 Auth 中间件注入操作人，不再解析本地 JWT。
		r.UpdateBy = middleware.OperatorName(ctx)

		return services.NoticeTmplService.Create(r)
	})
}

func (noticeTemplateController noticeTemplateController) Update(ctx *gin.Context) {
	r := new(types.RequestNoticeTemplateUpdate)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		r.UpdateBy = middleware.OperatorName(ctx)

		return services.NoticeTmplService.Update(r)
	})
}

func (noticeTemplateController noticeTemplateController) Delete(ctx *gin.Context) {
	r := new(types.RequestNoticeTemplateQuery)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		return services.NoticeTmplService.Delete(r)
	})
}

func (noticeTemplateController noticeTemplateController) List(ctx *gin.Context) {
	r := new(types.RequestNoticeTemplateQuery)
	BindQuery(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		return services.NoticeTmplService.List(r)
	})
}

func (noticeTemplateController noticeTemplateController) Get(ctx *gin.Context) {
	r := new(types.RequestNoticeTemplateQuery)
	BindQuery(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		return services.NoticeTmplService.Get(r)
	})
}
