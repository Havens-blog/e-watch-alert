package api

import (
	"watchAlert/internal/health"
	"watchAlert/pkg/response"

	"github.com/gin-gonic/gin"
)

type authHealthController struct{}

var AuthHealthController = new(authHealthController)

// Health 返回聚合降级状态（探针 A/B + 写禁用标记），供前端轮询驱动降级 UI。
func (authHealthController) Health(ctx *gin.Context) {
	response.Envelope(ctx, health.Snapshot(), nil)
}
