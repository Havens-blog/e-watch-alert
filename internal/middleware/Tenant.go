package middleware

import (
	"github.com/gin-gonic/gin"
	"watchAlert/pkg/response"
)

const TenantIDHeaderKey = "TenantID"

// ParseTenant 从 HTTP 头部获取 TenantID 并存储到上下文中。
//
// 租户真相源已切到 eiam：TenantID header 的值即 eiam 租户 id（int64 字符串，如 "2"），
// 由 eiam 共享会话在 Auth() 中间件中完成归属鉴权，此处不再查本地 tenants 表
// （本地租户表已随「租户表迁移到 eiam id」废弃）。
func ParseTenant() gin.HandlerFunc {
	return func(context *gin.Context) {
		tid := context.Request.Header.Get(TenantIDHeaderKey)
		if tid == "" {
			response.Fail(context, "租户ID不能为空", "failed")
			context.Abort()
			return
		}

		context.Set(TenantIDHeaderKey, tid)
		context.Next()
	}
}
