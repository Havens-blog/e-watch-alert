package services

import (
	"time"

	"watchAlert/internal/ctx"
	"watchAlert/internal/models"
	"watchAlert/internal/types"
	"watchAlert/pkg/response"
	"watchAlert/pkg/tools"
)

type migrationService struct {
	ctx *ctx.Context
}

type InterMigrationService interface {
	Export() (*types.ExportConfig, error)
	Import(req interface{}) (interface{}, error)
}

func newInterMigrationService(ctx *ctx.Context) InterMigrationService {
	return &migrationService{ctx: ctx}
}

// Export 导出全量规则/数据源/通知（不落历史事件；tenantId 置空；凭据字段原样透传）。
func (s migrationService) Export() (*types.ExportConfig, error) {
	rules, _, err := s.ctx.DB.Rule().List("", "", "", "", "", models.Page{Index: 0, Size: 1000000})
	if err != nil {
		return nil, err
	}
	datasources, err := s.ctx.DB.Datasource().List("", "", "", "")
	if err != nil {
		return nil, err
	}
	notifications, err := s.ctx.DB.Notice().List("", "", "")
	if err != nil {
		return nil, err
	}

	// tenantId 导出置空（跨租户迁移，导入时填目标租户）。
	for i := range rules {
		rules[i].TenantId = ""
	}
	for i := range datasources {
		datasources[i].TenantId = ""
	}
	for i := range notifications {
		notifications[i].TenantId = ""
	}

	return &types.ExportConfig{
		SchemaVersion: types.SchemaVersion,
		Rules:         rules,
		Datasources:   datasources,
		Notifications: notifications,
		ExportedAt:    time.Now().Format(time.RFC3339),
	}, nil
}

// Import 按 section（rules→datasources→notifications）逐条导入，含部分失败协议（§9）。
// 单条失败：记 ImportFailure 并中止当前 section（已成功保留、未处理跳过）；continue/abort 控制整批回滚。
func (s migrationService) Import(req interface{}) (interface{}, error) {
	r := req.(*types.ImportConfigReq)
	onFailure := r.OnFailure
	if onFailure == "" {
		onFailure = "abort"
	}
	if onFailure != "continue" && onFailure != "abort" {
		return nil, response.NewValidationError("on_failure 仅支持 continue|abort")
	}
	if r.Config.SchemaVersion != types.SchemaVersion {
		return nil, response.NewValidationError("schema_version 不兼容")
	}

	cfg := r.Config
	total := len(cfg.Rules) + len(cfg.Datasources) + len(cfg.Notifications)
	result := types.ImportResult{Total: total, Failures: []types.ImportFailure{}}

	// 记录本请求成功创建的条目 key，供超阈值回滚（补偿删除）。
	var createdRules, createdDatasources, createdNotices []string

	// rules
	for i := range cfg.Rules {
		rule := &cfg.Rules[i]
		rule.TenantId = "" // 导入填目标租户（基线空租户）
		if s.ctx.DB.Rule().GetRuleIsExist(rule.RuleId) {
			result.Succeeded++ // 幂等跳过
			continue
		}
		if reason := validateRule(rule); reason != "" {
			result.Failed++
			result.Failures = append(result.Failures, types.ImportFailure{Section: types.SectionRules, Index: i, Name: rule.RuleName, Reason: reason})
			break
		}
		if err := s.ctx.DB.Rule().Create(*rule); err != nil {
			result.Failed++
			result.Failures = append(result.Failures, types.ImportFailure{Section: types.SectionRules, Index: i, Name: rule.RuleName, Reason: "写入失败: " + err.Error()})
			break
		}
		createdRules = append(createdRules, rule.RuleId)
		result.Succeeded++
	}

	// datasources
	for i := range cfg.Datasources {
		ds := &cfg.Datasources[i]
		ds.TenantId = ""
		if s.datasourceExists(ds.ID) {
			result.Succeeded++
			continue
		}
		if reason := validateDatasource(ds); reason != "" {
			result.Failed++
			result.Failures = append(result.Failures, types.ImportFailure{Section: types.SectionDatasources, Index: i, Name: ds.Name, Reason: reason})
			break
		}
		if err := s.ctx.DB.Datasource().Create(*ds); err != nil {
			result.Failed++
			result.Failures = append(result.Failures, types.ImportFailure{Section: types.SectionDatasources, Index: i, Name: ds.Name, Reason: "写入失败: " + err.Error()})
			break
		}
		createdDatasources = append(createdDatasources, ds.ID)
		result.Succeeded++
	}

	// notifications
	for i := range cfg.Notifications {
		n := &cfg.Notifications[i]
		n.TenantId = ""
		if n.Uuid == "" {
			n.Uuid = tools.RandUid() // 未指定则导入侧生成
		}
		if s.noticeExists(n.Uuid) {
			result.Succeeded++
			continue
		}
		if reason := validateNotice(n); reason != "" {
			result.Failed++
			result.Failures = append(result.Failures, types.ImportFailure{Section: types.SectionNotifications, Index: i, Name: n.Name, Reason: reason})
			break
		}
		if err := s.ctx.DB.Notice().Create(*n); err != nil {
			result.Failed++
			result.Failures = append(result.Failures, types.ImportFailure{Section: types.SectionNotifications, Index: i, Name: n.Name, Reason: "写入失败: " + err.Error()})
			break
		}
		createdNotices = append(createdNotices, n.Uuid)
		result.Succeeded++
	}

	// 回滚选项：on_failure=abort 且失败率 > 20% → 补偿删除本请求成功创建的条目（整批回滚）。
	if onFailure == "abort" && result.Total > 0 && float64(result.Failed)/float64(result.Total) > 0.20 {
		s.rollback(createdRules, createdDatasources, createdNotices)
		result.Succeeded -= len(createdRules) + len(createdDatasources) + len(createdNotices)
	}

	return result, nil
}

func (s migrationService) datasourceExists(id string) bool {
	_, err := s.ctx.DB.Datasource().Get(id)
	return err == nil
}

func (s migrationService) noticeExists(uuid string) bool {
	_, err := s.ctx.DB.Notice().Get("", uuid)
	return err == nil
}

func (s migrationService) rollback(ruleIds, datasourceIds, noticeIds []string) {
	for _, id := range ruleIds {
		_ = s.ctx.DB.Rule().Delete("", id)
	}
	for _, id := range datasourceIds {
		_ = s.ctx.DB.Datasource().Delete("", id)
	}
	for _, id := range noticeIds {
		_ = s.ctx.DB.Notice().Delete("", id)
	}
}

// validateRule 返回失败原因（空串=通过）。
func validateRule(r *models.AlertRule) string {
	if r.RuleId == "" {
		return "字段缺失: ruleId"
	}
	if r.RuleName == "" {
		return "字段缺失: ruleName"
	}
	if r.EvalInterval < 5 {
		return "字段非法: evalInterval < 5"
	}
	return ""
}

func validateDatasource(d *models.AlertDataSource) string {
	if d.ID == "" {
		return "字段缺失: id"
	}
	if d.Name == "" {
		return "字段缺失: name"
	}
	return ""
}

func validateNotice(n *models.AlertNotice) string {
	if n.Name == "" {
		return "字段缺失: name"
	}
	return ""
}
