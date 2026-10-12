package middleware

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"watchAlert/internal/ctx"
	"watchAlert/internal/health"
	"watchAlert/pkg/response"
	"watchAlert/pkg/tools"

	"github.com/ecodeclub/ginx/gctx"
	"github.com/ecodeclub/ginx/session"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const ApiKeyHeader = "X-API-Key"

var (
	// sessionProvider 共享会话 provider（main.go InitSession 注入；与 eiam 同款 ginx/session）。
	sessionProvider session.Provider
	// sessionCache 进程内会话缓存兜底（Redis 抖动时读接口降级）。
	sessionCache = NewSessionCache(1024, 30*time.Second)
)

// InitSession 注入共享会话 provider（main.go 启动时调用）。
func InitSession(provider session.Provider) {
	sessionProvider = provider
}

// Auth 共享会话校验中间件（tech-design §3.4）：
// X-API-Key 机器通道 → 读共享会话 → Redis 存活校验 → eiam_uid 角色映射 → 放行注入 UserId。
// 按序分派：无/过期会话 401、无角色绑定 403、Redis 不可达 503（写接口）或 SessionCache（读接口）、放行 200。
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. X-API-Key 机器鉴权通道（保留，不查角色绑定）
		if apiKey := c.GetHeader(ApiKeyHeader); apiKey != "" {
			userId, ok := IsApiKeyValid(ctx.DO(), apiKey)
			if !ok {
				response.Envelope(c, nil, response.NewUnauthorizedError("未认证或会话过期"))
				c.Abort()
				return
			}
			c.Set("UserId", userId)
			c.Next()
			return
		}

		// 2. 降级：写接口统一 503（规则/静默/通知/数据源/确认；探针 A/B 任一 degraded）
		if isWriteRequest(c) && health.WriteDisabled() {
			response.Envelope(c, nil, response.NewUnavailableError("鉴权服务暂不可用"))
			c.Abort()
			return
		}

		// 3. 读共享会话（验签 JWT，无 Redis 读）
		if sessionProvider == nil {
			response.Envelope(c, nil, response.NewUnavailableError("鉴权服务暂不可用"))
			c.Abort()
			return
		}
		sess, err := sessionProvider.Get(&gctx.Context{Context: c})
		if err != nil {
			// 非共享会话：过渡期回退旧本地 JWT（验签只读副本）
			if uid, ok := tryLegacyJWT(c); ok {
				if isWriteRequest(c) {
					// 旧 JWT 命中写方法 → 403（写路径自切换日起强制走共享会话）
					response.Envelope(c, nil, response.NewForbiddenError("旧会话已失效，请重新登录"))
					c.Abort()
				} else {
					// 读接口容忍旧 JWT 至过期
					c.Set("UserId", uid)
					c.Next()
				}
				return
			}
			// 无/过期/无效 token → 401（不跳登录由前端拦截器处理）
			response.Envelope(c, nil, response.NewUnauthorizedError("未认证或会话过期"))
			c.Abort()
			return
		}

		uid := sess.Claims().Uid

		// 3. Redis 存活校验（处理单点登出销毁会话）
		if err := checkSessionAlive(c, sess); err != nil {
			response.Envelope(c, nil, err)
			c.Abort()
			return
		}

		// 4. eiam_uid → 内部角色映射
		rb, exists, err := ctx.DO().DB.RoleBinding().GetByUid(uid)
		if err != nil {
			response.Envelope(c, nil, response.NewInternalError("服务器内部错误"))
			c.Abort()
			return
		}
		if !exists {
			// 会话有效但无角色绑定 → 403（不跳登录）
			response.Envelope(c, nil, response.NewForbiddenError("无访问权限"))
			c.Abort()
			return
		}

		// 5. 注入 UserId（兼容下游 string 断言）+ Role + EiamUid
		// 人员用户 id 已统一为 eiam uid（members.user_id 迁移到 eiam uid），
		// 不再做 admin -> 本地 "admin" 的特殊映射，admin 即 eiam uid 字符串（如 "1"）。
		userId := strconv.FormatInt(uid, 10)
		c.Set("UserId", userId)
		c.Set("Role", rb.Role)
		c.Set("EiamUid", uid)

		// 6. 写缓存（cache-aside：成功校验后写一次，吸收后续 Redis 抖动）
		sessionCache.Set(sess.Claims().SSID, sess.Claims())

		c.Next()
	}
}

// checkSessionAlive 校验会话在共享 Redis 中是否仍存活。
// 返回 *response.ApiError：401（已销毁）/ 503（Redis 不可达且缓存未命中）；nil 表示存活或读接口缓存兜底命中。
func checkSessionAlive(c *gin.Context, sess session.Session) error {
	av := sess.Get(c.Request.Context(), "uid")
	if av.Err == nil {
		health.RecordProbeB(true)
		return nil // 存活
	}

	if errors.Is(av.Err, redis.Nil) {
		// 会话已销毁（logout）→ 401（Redis 本身可达，探针 B 记成功）
		health.RecordProbeB(true)
		return response.NewUnauthorizedError("未认证或会话过期")
	}

	// Redis 连接失败/超时：探针 B 记失败
	health.RecordProbeB(false)
	if isWriteRequest(c) {
		// 写接口一律不查缓存，强制实时校验 → 503
		return response.NewUnavailableError("鉴权服务暂不可用")
	}
	// 读接口走 SessionCache 兜底：命中放行、未命中/过期 → 503
	if _, ok := sessionCache.Get(sess.Claims().SSID); ok {
		return nil
	}
	return response.NewUnavailableError("鉴权服务暂不可用")
}

// isWriteRequest 判定写请求（非 GET/HEAD/OPTIONS 一律视为写）。
func isWriteRequest(c *gin.Context) bool {
	switch c.Request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

// tryLegacyJWT 过渡期回退验签旧本地 JWT（只读副本），命中返回旧 userId。
func tryLegacyJWT(c *gin.Context) (string, bool) {
	uid := tools.GetUserID(c.GetHeader("Authorization"))
	if uid == "" {
		return "", false
	}
	return uid, true
}

// IsApiKeyValid 验证 API 密钥（X-API-Key 机器鉴权通道，保留原逻辑）。
func IsApiKeyValid(ctx *ctx.Context, apiKey string) (string, bool) {
	apiKeyModel, exists, err := ctx.DB.ApiKey().GetByKey(apiKey)
	if err != nil || !exists {
		return "无效的API密钥", false
	}
	return apiKeyModel.UserId, true
}
