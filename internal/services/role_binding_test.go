package services

import (
	"context"
	"path/filepath"
	"testing"

	"watchAlert/config"
	"watchAlert/internal/ctx"
	"watchAlert/internal/repo"
	"watchAlert/internal/types"
	"watchAlert/pkg/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRoleBindingService 用 SQLite 临时库 + 真实 repo 构建 service。
// 返回 cleanup 用于关闭 SQLite 连接（否则 Windows 下 TempDir 清理会失败）。
func newTestRoleBindingService(t *testing.T) (InterRoleBindingService, func()) {
	t.Helper()
	config.Application.Database.Type = "sqlite"
	config.Application.Database.Path = filepath.Join(t.TempDir(), "rolebinding_test.db")
	config.Application.Server.Mode = "test" // 关闭 db.Debug() 噪声

	repoEntry := repo.NewRepoEntry()
	require.NotNil(t, repoEntry, "repo 初始化失败")

	cleanup := func() {
		if sqlDB, err := repoEntry.DB().DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	t.Cleanup(cleanup)

	c := ctx.NewContext(context.Background(), repoEntry, nil)
	return newInterRoleBindingService(c), cleanup
}

func TestRoleBindingService_Create_Success(t *testing.T) {
	svc, _ := newTestRoleBindingService(t)

	req := &types.RequestRoleBindingCreate{
		Bindings: []types.RoleBindingItem{
			{EiamUid: 1001, Role: "admin"},
			{EiamUid: 1002, Role: "user"},
		},
	}
	data, err := svc.Create(req)
	require.NoError(t, err)

	items, ok := data.([]types.RoleBindingItemV0)
	require.True(t, ok)
	require.Len(t, items, 2)
	assert.Equal(t, int64(1001), items[0].EiamUid)
	assert.Equal(t, "admin", items[0].Role)
	assert.Equal(t, int64(0), items[0].Version)
}

func TestRoleBindingService_Create_Validation(t *testing.T) {
	svc, _ := newTestRoleBindingService(t)

	cases := []struct {
		name    string
		binding types.RoleBindingItem
	}{
		{"role 非法", types.RoleBindingItem{EiamUid: 1001, Role: "superadmin"}},
		{"role 空", types.RoleBindingItem{EiamUid: 1001, Role: ""}},
		{"eiam_uid 非正", types.RoleBindingItem{EiamUid: 0, Role: "admin"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(&types.RequestRoleBindingCreate{Bindings: []types.RoleBindingItem{tc.binding}})
			require.Error(t, err)
			apiErr, ok := err.(*response.ApiError)
			require.True(t, ok)
			assert.Equal(t, 1400, apiErr.Code)
			assert.Equal(t, 400, apiErr.HTTPStatus)
		})
	}

	// bindings 为空
	_, err := svc.Create(&types.RequestRoleBindingCreate{Bindings: []types.RoleBindingItem{}})
	require.Error(t, err)
	assert.Equal(t, 1400, err.(*response.ApiError).Code)
}

func TestRoleBindingService_Create_Conflict(t *testing.T) {
	svc, _ := newTestRoleBindingService(t)

	// 首次创建 version=0
	_, err := svc.Create(&types.RequestRoleBindingCreate{
		Bindings: []types.RoleBindingItem{{EiamUid: 1001, Role: "admin"}},
	})
	require.NoError(t, err)

	// 携带过期 version=5（实际已是 0）→ 409
	v := int64(5)
	_, err = svc.Create(&types.RequestRoleBindingCreate{
		Bindings: []types.RoleBindingItem{{EiamUid: 1001, Role: "user", Version: &v}},
	})
	require.Error(t, err)
	apiErr, ok := err.(*response.ApiError)
	require.True(t, ok)
	assert.Equal(t, 1409, apiErr.Code)
	assert.Equal(t, 409, apiErr.HTTPStatus)
}

func TestRoleBindingService_List(t *testing.T) {
	svc, _ := newTestRoleBindingService(t)

	_, err := svc.Create(&types.RequestRoleBindingCreate{
		Bindings: []types.RoleBindingItem{
			{EiamUid: 1001, Role: "admin"},
			{EiamUid: 1002, Role: "user"},
		},
	})
	require.NoError(t, err)

	data, err := svc.List(&types.RequestRoleBindingList{Offset: 0, Limit: 10, Keyword: ""})
	require.NoError(t, err)
	page, ok := data.(types.ResponseRoleBindingPage)
	require.True(t, ok)
	assert.Equal(t, int64(2), page.Total)
	assert.Len(t, page.Items, 2)
}

func TestRoleBindingService_Delete(t *testing.T) {
	svc, _ := newTestRoleBindingService(t)

	_, err := svc.Create(&types.RequestRoleBindingCreate{
		Bindings: []types.RoleBindingItem{{EiamUid: 1001, Role: "admin"}},
	})
	require.NoError(t, err)

	// 删除存在的
	data, err := svc.Delete(1001)
	require.NoError(t, err)
	assert.NotNil(t, data)

	// 删除不存在的 → 404
	_, err = svc.Delete(9999)
	require.Error(t, err)
	apiErr, ok := err.(*response.ApiError)
	require.True(t, ok)
	assert.Equal(t, 1404, apiErr.Code)
}
