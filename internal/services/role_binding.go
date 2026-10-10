package services

import (
	"errors"
	"time"

	"watchAlert/internal/ctx"
	"watchAlert/internal/models"
	"watchAlert/internal/repo"
	"watchAlert/internal/types"
	"watchAlert/pkg/response"

	"gorm.io/gorm"
)

type roleBindingService struct {
	ctx *ctx.Context
}

type InterRoleBindingService interface {
	Create(req interface{}) (interface{}, error)
	List(req interface{}) (interface{}, error)
	Delete(eiamUid int64) (interface{}, error)
}

func newInterRoleBindingService(ctx *ctx.Context) InterRoleBindingService {
	return &roleBindingService{ctx: ctx}
}

func (s roleBindingService) Create(req interface{}) (interface{}, error) {
	r := req.(*types.RequestRoleBindingCreate)
	if len(r.Bindings) == 0 {
		return nil, response.NewValidationError("bindings 不能为空")
	}

	items := make([]types.RoleBindingItemV0, 0, len(r.Bindings))
	for _, item := range r.Bindings {
		if item.EiamUid <= 0 {
			return nil, response.NewValidationError("eiam_uid 校验失败")
		}
		if !models.IsValidRole(item.Role) {
			return nil, response.NewValidationError("role 校验失败")
		}

		binding := models.RoleBinding{EiamUid: item.EiamUid, Role: item.Role}
		var saved models.RoleBinding
		var err error
		if item.Version != nil {
			saved, err = s.ctx.DB.RoleBinding().UpsertWithVersion(binding, *item.Version)
			if errors.Is(err, repo.ErrRoleBindingConflict) {
				return nil, response.NewConflictError("数据已被他人修改，请刷新后重试")
			}
		} else {
			saved, err = s.ctx.DB.RoleBinding().Upsert(binding)
		}
		if err != nil {
			return nil, err
		}
		items = append(items, toRoleBindingItem(saved))
	}
	return items, nil
}

func (s roleBindingService) List(req interface{}) (interface{}, error) {
	r := req.(*types.RequestRoleBindingList)
	if r.Limit <= 0 || r.Limit > 1000 {
		r.Limit = 20
	}
	if r.Offset < 0 {
		r.Offset = 0
	}

	data, total, err := s.ctx.DB.RoleBinding().List(r.Offset, r.Limit, r.Keyword)
	if err != nil {
		return nil, err
	}

	items := make([]types.RoleBindingItemV0, 0, len(data))
	for _, item := range data {
		items = append(items, toRoleBindingItem(item))
	}
	return types.ResponseRoleBindingPage{Total: total, Items: items}, nil
}

func (s roleBindingService) Delete(eiamUid int64) (interface{}, error) {
	err := s.ctx.DB.RoleBinding().Delete(eiamUid)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, response.NewNotFoundError("资源不存在")
	}
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{}, nil
}

func toRoleBindingItem(m models.RoleBinding) types.RoleBindingItemV0 {
	return types.RoleBindingItemV0{
		EiamUid:   m.EiamUid,
		Role:      m.Role,
		Version:   m.Version,
		CreatedAt: m.CreatedAt.Format(time.RFC3339),
		UpdatedAt: m.UpdatedAt.Format(time.RFC3339),
	}
}
