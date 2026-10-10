package types

import (
	"watchAlert/internal/models"
)

// SchemaVersion 迁移配置 schevers 版本锁（部署基线保证两端模板/结构一致）。
const SchemaVersion = "watchalert/v1"

// ExportConfig 迁移导出载荷（§3.4：三模型全字段 JSON，json tag 原样，仅 tenantId 例外）。
type ExportConfig struct {
	SchemaVersion string                   `json:"schema_version"`
	Rules         []models.AlertRule       `json:"rules"`
	Datasources   []models.AlertDataSource `json:"datasources"`
	Notifications []models.AlertNotice     `json:"notifications"`
	ExportedAt    string                   `json:"exported_at"`
}

// ImportConfigReq 迁移导入请求（§3.4/§9）。
type ImportConfigReq struct {
	Config    ExportConfig `json:"config"`
	OnFailure string       `json:"on_failure"` // "continue" | "abort"（默认 "abort"）
}

// ImportFailure 单条导入失败信息（§3.4）。
type ImportFailure struct {
	Section string `json:"section"` // rules | datasources | notifications
	Index   int    `json:"index"`   // 在所属 section 数组中的下标
	Name    string `json:"name"`
	Reason  string `json:"reason"`
}

// ImportResult 导入结果（§3.4）。
type ImportResult struct {
	Total     int             `json:"total"`
	Succeeded int             `json:"succeeded"`
	Failed    int             `json:"failed"`
	Failures  []ImportFailure `json:"failures"`
}

// 迁移 section 常量（与 ImportFailure.Section 对应）。
const (
	SectionRules         = "rules"
	SectionDatasources   = "datasources"
	SectionNotifications = "notifications"
)
