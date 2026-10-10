package services

import (
	"context"
	"path/filepath"
	"testing"

	"watchAlert/config"
	"watchAlert/internal/ctx"
	"watchAlert/internal/models"
	"watchAlert/internal/repo"
	"watchAlert/internal/types"
	"watchAlert/pkg/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestMigrationService(t *testing.T) (InterMigrationService, func()) {
	t.Helper()
	config.Application.Database.Type = "sqlite"
	config.Application.Database.Path = filepath.Join(t.TempDir(), "migration_test.db")
	config.Application.Server.Mode = "test"

	repoEntry := repo.NewRepoEntry()
	require.NotNil(t, repoEntry, "repo 初始化失败")

	cleanup := func() {
		if sqlDB, err := repoEntry.DB().DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	t.Cleanup(cleanup)

	c := ctx.NewContext(context.Background(), repoEntry, nil)
	return newInterMigrationService(c), cleanup
}

func enabledPtr(b bool) *bool { return &b }

func buildSampleConfig() *types.ExportConfig {
	rulesEnabled := enabledPtr(true)
	enabled := enabledPtr(true)
	return &types.ExportConfig{
		SchemaVersion: types.SchemaVersion,
		Rules: []models.AlertRule{
			{
				TenantId:     "t1",
				RuleId:       "rule-1",
				RuleName:     "rule one",
				EvalInterval: 30,
				Severity:     "critical",
				Enabled:      rulesEnabled,
			},
			{
				TenantId:     "t1",
				RuleId:       "rule-2",
				RuleName:     "rule two",
				EvalInterval: 60,
				Severity:     "warning",
				Enabled:      rulesEnabled,
			},
		},
		Datasources: []models.AlertDataSource{
			{
				TenantId: "t1",
				ID:       "ds-1",
				Name:     "prom",
				Type:     "prometheus",
				Enabled:  enabled,
				Auth:     models.Auth{User: "u1", Pass: "secret-pass-123"},
			},
		},
		Notifications: []models.AlertNotice{
			{
				TenantId: "t1",
				Uuid:     "n-1",
				Name:     "notify one",
				Routes: []models.Route{
					{NoticeType: "webhook", Hook: "https://hooks.example.com/secret", Sign: "sig-secret"},
				},
			},
		},
	}
}

func TestMigration_ExportImport_RoundTrip(t *testing.T) {
	svc, _ := newTestMigrationService(t)
	cfg := buildSampleConfig()

	// 导入
	res, err := svc.Import(&types.ImportConfigReq{Config: *cfg, OnFailure: "continue"})
	require.NoError(t, err)
	result := res.(types.ImportResult)
	assert.Equal(t, 4, result.Total)
	assert.Equal(t, 4, result.Succeeded)
	assert.Equal(t, 0, result.Failed)
	assert.Len(t, result.Failures, 0)

	// 导出并对比（tenantId 期望置空）
	exported, err := svc.Export()
	require.NoError(t, err)
	assert.Equal(t, types.SchemaVersion, exported.SchemaVersion)
	require.Len(t, exported.Rules, 2)
	require.Len(t, exported.Datasources, 1)
	require.Len(t, exported.Notifications, 1)

	// 规整 tenantId 后对比（导出置空）
	cfg.Rules[0].TenantId = ""
	cfg.Rules[1].TenantId = ""
	cfg.Datasources[0].TenantId = ""
	cfg.Notifications[0].TenantId = ""

	assert.Equal(t, cfg.Rules, exported.Rules)
	assert.Equal(t, cfg.Datasources, exported.Datasources)
	assert.Equal(t, cfg.Notifications, exported.Notifications)
}

func TestMigration_Import_PartialFailure_Continue(t *testing.T) {
	svc, _ := newTestMigrationService(t)
	cfg := buildSampleConfig()

	// 注入 1 条坏规则（ruleName 缺失）到 rules 末尾
	cfg.Rules = append(cfg.Rules, models.AlertRule{TenantId: "t1", RuleId: "bad", RuleName: "", EvalInterval: 30})

	res, err := svc.Import(&types.ImportConfigReq{Config: *cfg, OnFailure: "continue"})
	require.NoError(t, err)
	result := res.(types.ImportResult)

	// 4 valid + 1 bad = 5 total；坏条目记入失败清单、其余成功
	assert.Equal(t, 5, result.Total)
	assert.Equal(t, 4, result.Succeeded)
	assert.Equal(t, 1, result.Failed)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, types.SectionRules, result.Failures[0].Section)
	assert.Equal(t, 2, result.Failures[0].Index)
	assert.Equal(t, "rules", result.Failures[0].Section)
}

func TestMigration_Import_SchemaIncompat(t *testing.T) {
	svc, _ := newTestMigrationService(t)
	cfg := buildSampleConfig()
	cfg.SchemaVersion = "watchalert/v99"

	_, err := svc.Import(&types.ImportConfigReq{Config: *cfg, OnFailure: "abort"})
	require.Error(t, err)
	apiErr, ok := err.(*response.ApiError)
	require.True(t, ok)
	assert.Equal(t, 1400, apiErr.Code)

	// 无任何导入
	exported, err := svc.Export()
	require.NoError(t, err)
	assert.Len(t, exported.Rules, 0)
	assert.Len(t, exported.Datasources, 0)
	assert.Len(t, exported.Notifications, 0)
}

func TestMigration_CredentialPassthrough(t *testing.T) {
	svc, _ := newTestMigrationService(t)
	cfg := buildSampleConfig()

	_, err := svc.Import(&types.ImportConfigReq{Config: *cfg, OnFailure: "continue"})
	require.NoError(t, err)

	exported, err := svc.Export()
	require.NoError(t, err)
	require.Len(t, exported.Datasources, 1)
	require.Len(t, exported.Notifications, 1)

	// 凭据字段原样透传（当前代码库无主密钥加密，CredentialField=string 透传）
	assert.Equal(t, "secret-pass-123", exported.Datasources[0].Auth.Pass)
	assert.Equal(t, "https://hooks.example.com/secret", exported.Notifications[0].Routes[0].Hook)
	assert.Equal(t, "sig-secret", exported.Notifications[0].Routes[0].Sign)
}

func TestMigration_Import_AbortRollback(t *testing.T) {
	svc, _ := newTestMigrationService(t)

	// 2 条有效 + 1 条坏（末尾）→ 失败率 1/3 > 20% → on_failure=abort 整批回滚
	cfg := &types.ExportConfig{
		SchemaVersion: types.SchemaVersion,
		Rules: []models.AlertRule{
			{TenantId: "t1", RuleId: "r1", RuleName: "ok one", EvalInterval: 30},
			{TenantId: "t1", RuleId: "r2", RuleName: "ok two", EvalInterval: 30},
			{TenantId: "t1", RuleId: "bad", RuleName: "", EvalInterval: 30},
		},
	}

	res, err := svc.Import(&types.ImportConfigReq{Config: *cfg, OnFailure: "abort"})
	require.NoError(t, err)
	result := res.(types.ImportResult)
	assert.Equal(t, 3, result.Total)
	assert.Equal(t, 1, result.Failed)
	assert.Len(t, result.Failures, 1)

	// 整批回滚后无残留
	exported, err := svc.Export()
	require.NoError(t, err)
	assert.Len(t, exported.Rules, 0, ">20% 失败应整批回滚")
}

func TestMigration_Import_OnFailureInvalid(t *testing.T) {
	svc, _ := newTestMigrationService(t)
	cfg := buildSampleConfig()

	_, err := svc.Import(&types.ImportConfigReq{Config: *cfg, OnFailure: "bad"})
	require.Error(t, err)
	apiErr, ok := err.(*response.ApiError)
	require.True(t, ok)
	assert.Equal(t, 1400, apiErr.Code)
}

func TestMigration_Import_Idempotent(t *testing.T) {
	svc, _ := newTestMigrationService(t)
	cfg := buildSampleConfig()

	// 首次导入：4 成功
	res, err := svc.Import(&types.ImportConfigReq{Config: *cfg, OnFailure: "continue"})
	require.NoError(t, err)
	assert.Equal(t, 4, res.(types.ImportResult).Succeeded)

	// 增量重导：同 payload，全部幂等跳过（不重复创建）
	res2, err := svc.Import(&types.ImportConfigReq{Config: *cfg, OnFailure: "continue"})
	require.NoError(t, err)
	r2 := res2.(types.ImportResult)
	assert.Equal(t, 4, r2.Succeeded)
	assert.Equal(t, 0, r2.Failed)

	exported, err := svc.Export()
	require.NoError(t, err)
	assert.Len(t, exported.Rules, 2, "重导不应重复创建")
	assert.Len(t, exported.Datasources, 1)
	assert.Len(t, exported.Notifications, 1)
}

func TestMigration_Import_RuleValidation(t *testing.T) {
	svc, _ := newTestMigrationService(t)
	cfg := &types.ExportConfig{
		SchemaVersion: types.SchemaVersion,
		Rules: []models.AlertRule{
			{TenantId: "t1", RuleId: "", RuleName: "no-id", EvalInterval: 30}, // 缺 ruleId
			{TenantId: "t1", RuleId: "r2", RuleName: "no", EvalInterval: 1},   // evalInterval<5
		},
	}

	res, err := svc.Import(&types.ImportConfigReq{Config: *cfg, OnFailure: "continue"})
	require.NoError(t, err)
	result := res.(types.ImportResult)
	assert.Equal(t, 1, result.Failed) // 首条失败即中止本节
	require.Len(t, result.Failures, 1)
	assert.Contains(t, result.Failures[0].Reason, "ruleId")
}

func TestMigration_Import_DatasourceValidation(t *testing.T) {
	cases := []struct {
		name string
		ds   models.AlertDataSource
		want string
	}{
		{"缺 id", models.AlertDataSource{TenantId: "t1", ID: "", Name: "n"}, "id"},
		{"缺 name", models.AlertDataSource{TenantId: "t1", ID: "ds-1", Name: ""}, "name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newTestMigrationService(t)
			cfg := &types.ExportConfig{
				SchemaVersion: types.SchemaVersion,
				Datasources:   []models.AlertDataSource{tc.ds},
			}
			res, err := svc.Import(&types.ImportConfigReq{Config: *cfg, OnFailure: "continue"})
			require.NoError(t, err)
			result := res.(types.ImportResult)
			require.Len(t, result.Failures, 1)
			assert.Contains(t, result.Failures[0].Reason, tc.want)
		})
	}
}

func TestMigration_Import_NoticeValidation(t *testing.T) {
	svc, _ := newTestMigrationService(t)
	cfg := &types.ExportConfig{
		SchemaVersion: types.SchemaVersion,
		Notifications: []models.AlertNotice{{TenantId: "t1", Uuid: "n-1", Name: ""}},
	}
	res, err := svc.Import(&types.ImportConfigReq{Config: *cfg, OnFailure: "continue"})
	require.NoError(t, err)
	result := res.(types.ImportResult)
	require.Len(t, result.Failures, 1)
	assert.Contains(t, result.Failures[0].Reason, "name")
}

func TestMigration_Import_RollbackMixedTypes(t *testing.T) {
	svc, _ := newTestMigrationService(t)
	enabled := enabledPtr(true)

	// 1 rule + 1 datasource + 2 notices（1 有效 + 1 坏）→ 失败率 1/4 = 25% > 20% → 回滚（覆盖三类删除）
	cfg := &types.ExportConfig{
		SchemaVersion: types.SchemaVersion,
		Rules:         []models.AlertRule{{TenantId: "t1", RuleId: "r1", RuleName: "ok", EvalInterval: 30}},
		Datasources:   []models.AlertDataSource{{TenantId: "t1", ID: "ds-1", Name: "ds", Enabled: enabled}},
		Notifications: []models.AlertNotice{
			{TenantId: "t1", Uuid: "n-1", Name: "ok"},
			{TenantId: "t1", Uuid: "n-2", Name: ""}, // 坏
		},
	}

	res, err := svc.Import(&types.ImportConfigReq{Config: *cfg, OnFailure: "abort"})
	require.NoError(t, err)
	result := res.(types.ImportResult)
	assert.Equal(t, 1, result.Failed)

	// 三类均回滚删除
	exported, err := svc.Export()
	require.NoError(t, err)
	assert.Len(t, exported.Rules, 0)
	assert.Len(t, exported.Datasources, 0)
	assert.Len(t, exported.Notifications, 0)
}
