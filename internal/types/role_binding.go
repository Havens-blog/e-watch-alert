package types

// RoleBindingItem POST /api/watchalert/role-bindings 请求体里的单条绑定。
type RoleBindingItem struct {
	EiamUid int64  `json:"eiam_uid" binding:"required"`
	Role    string `json:"role" binding:"required"`
	Version *int64 `json:"version"` // 可选，携带则做乐观锁（不匹配 409）
}

// RequestRoleBindingCreate POST /api/watchalert/role-bindings。
type RequestRoleBindingCreate struct {
	Bindings []RoleBindingItem `json:"bindings" binding:"required,dive"`
}

// RequestRoleBindingList GET /api/watchalert/role-bindings?offset=&limit=&keyword=
type RequestRoleBindingList struct {
	Offset  int    `form:"offset"`
	Limit   int    `form:"limit"`
	Keyword string `form:"keyword"`
}

// ResponseRoleBindingPage 列表响应 data（TECH-api-002 Page<T>）。
type ResponseRoleBindingPage struct {
	Total int64               `json:"total"`
	Items []RoleBindingItemV0 `json:"items"`
}

// RoleBindingItemV0 列表/详情项（对齐设计 §3.4 RoleBinding：snake_case + RFC3339）。
type RoleBindingItemV0 struct {
	EiamUid   int64  `json:"eiam_uid"`
	Role      string `json:"role"`
	Version   int64  `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
