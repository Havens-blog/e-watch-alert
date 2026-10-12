package api

import (
	"errors"
	middleware "watchAlert/internal/middleware"
	"watchAlert/internal/services"
	"watchAlert/internal/types"

	"github.com/gin-gonic/gin"
)

type userController struct{}

var UserController = new(userController)

/*
用户 API
/api/w8t/user
*/
func (userController userController) API(gin *gin.RouterGroup) {

	a := gin.Group("user")
	a.Use(
		middleware.Auth(),
		middleware.Permission(),
		middleware.ParseTenant(),
		middleware.AuditingLog(),
	)
	{
		a.POST("userUpdate", userController.Update)
		a.POST("userDelete", userController.Delete)
		a.POST("userChangePass", userController.ChangePass)
		a.POST("userSyncLocal", userController.SyncLocal)
	}

	b := gin.Group("user")
	b.Use(
		middleware.Auth(),
		middleware.Permission(),
		middleware.ParseTenant(),
	)
	{
		b.GET("userList", userController.List)
	}

}

func (userController userController) List(ctx *gin.Context) {
	r := new(types.RequestUserQuery)
	BindQuery(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		return services.UserService.List(r)
	})
}

func (userController userController) GetUserInfo(ctx *gin.Context) {
	r := new(types.RequestUserQuery)

	Service(ctx, func() (interface{}, interface{}) {
		// 共享会话鉴权后由 Auth 中间件注入 UserId（eiam_uid 或 API-Key userId），
		// 不再读 Authorization 头（旧本地 JWT 已移除）。
		userIdVal, exists := ctx.Get("UserId")
		if !exists {
			return nil, errors.New("userId is empty")
		}
		userId, ok := userIdVal.(string)
		if !ok || userId == "" {
			return nil, errors.New("userId is empty")
		}
		r.UserId = userId

		return services.UserService.Info(r)
	})
}

func (userController userController) Register(ctx *gin.Context) {
	r := new(types.RequestUserCreate)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		createUser := middleware.OperatorName(ctx)
		r.CreateBy = createUser

		return services.UserService.Register(r)
	})
}

func (userController userController) Update(ctx *gin.Context) {
	r := new(types.RequestUserUpdate)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		return services.UserService.Update(r)
	})
}

func (userController userController) Delete(ctx *gin.Context) {
	r := new(types.RequestUserQuery)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		return services.UserService.Delete(r)
	})
}

// SyncLocal 同步 eiam 用户到本地扩展表（承载 join_duty 等应用侧扩展属性）。
func (userController userController) SyncLocal(ctx *gin.Context) {
	r := new(types.RequestUserSyncLocal)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		return services.UserService.SyncLocal(r)
	})
}

func (userController userController) CheckUser(ctx *gin.Context) {
	r := new(types.RequestUserQuery)
	BindQuery(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		return services.UserService.Check(r)
	})
}

func (userController userController) ChangePass(ctx *gin.Context) {
	r := new(types.RequestUserChangePassword)
	BindJson(ctx, r)

	Service(ctx, func() (interface{}, interface{}) {
		// 会话化后由 Auth 中间件注入 UserId（eiam uid 字符串），不再解析本地 JWT。
		userIDVal, exists := ctx.Get("UserId")
		if !exists {
			return nil, errors.New("UserID 不能为空")
		}
		userID, _ := userIDVal.(string)
		if userID == "" {
			return nil, errors.New("UserID 不能为空")
		}

		if userID != r.UserId {
			return nil, errors.New("只能修改自己的密码")
		}

		return services.UserService.ChangePass(r)
	})
}
