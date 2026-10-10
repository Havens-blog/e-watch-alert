package models

import "time"

// 角色绑定角色取值（与设计 §3.4 Role 枚举一致）。
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// RoleBinding eiam_uid → 内部角色（admin/user）桥接表（watchalert_role_binding）。
// 与共享会话鉴权配套：middleware.Auth() 按 eiam_uid 查本表做角色映射。
// version 为乐观锁版本号：POST/DELETE 携带 version 时按 WHERE eiam_uid=? AND version=? 更新，不匹配返回 409。
type RoleBinding struct {
	EiamUid   int64     `json:"eiam_uid" gorm:"column:eiam_uid;primaryKey"`
	Role      string    `json:"role" gorm:"column:role;size:16;not null"`
	Version   int64     `json:"version" gorm:"column:version;not null;default:0"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (RoleBinding) TableName() string {
	return "watchalert_role_binding"
}

// IsValidRole 校验角色枚举（admin/user），长度 ≤16。
func IsValidRole(role string) bool {
	return role == RoleAdmin || role == RoleUser
}
