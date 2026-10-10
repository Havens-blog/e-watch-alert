package repo

import (
	"errors"
	"strconv"
	"time"

	"watchAlert/internal/models"

	"gorm.io/gorm"
)

// ErrRoleBindingConflict 乐观锁 version 不匹配。
var ErrRoleBindingConflict = errors.New("role binding version conflict")

type (
	RoleBindingRepo struct {
		entryRepo
	}

	InterRoleBindingRepo interface {
		List(offset, limit int, keyword string) ([]models.RoleBinding, int64, error)
		GetByUid(eiamUid int64) (models.RoleBinding, bool, error)
		// Upsert 无乐观锁纯 upsert（用于首次 seed/批量预置），version 自增。
		Upsert(binding models.RoleBinding) (models.RoleBinding, error)
		// UpsertWithVersion 乐观锁：existing.version 必须 == expectedVersion，否则返回 ErrRoleBindingConflict。
		UpsertWithVersion(binding models.RoleBinding, expectedVersion int64) (models.RoleBinding, error)
		Delete(eiamUid int64) error
	}
)

func newRoleBindingInterface(db *gorm.DB, g InterGormDBCli) InterRoleBindingRepo {
	return &RoleBindingRepo{
		entryRepo{g: g, db: db},
	}
}

func (r RoleBindingRepo) List(offset, limit int, keyword string) ([]models.RoleBinding, int64, error) {
	var data []models.RoleBinding
	var total int64
	db := r.db.Model(&models.RoleBinding{})

	if keyword != "" {
		query := "role LIKE ?"
		args := []interface{}{"%" + keyword + "%"}
		if uid, err := strconv.ParseInt(keyword, 10, 64); err == nil {
			query += " OR eiam_uid = ?"
			args = append(args, uid)
		}
		db = db.Where(query, args...)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("eiam_uid ASC").Offset(offset).Limit(limit).Find(&data).Error; err != nil {
		return nil, 0, err
	}
	return data, total, nil
}

func (r RoleBindingRepo) GetByUid(eiamUid int64) (models.RoleBinding, bool, error) {
	var data models.RoleBinding
	err := r.db.Model(&models.RoleBinding{}).Where("eiam_uid = ?", eiamUid).First(&data).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return data, false, nil
		}
		return data, false, err
	}
	return data, true, nil
}

func (r RoleBindingRepo) Upsert(binding models.RoleBinding) (models.RoleBinding, error) {
	existing, exists, err := r.GetByUid(binding.EiamUid)
	if err != nil {
		return binding, err
	}
	if !exists {
		return r.create(binding)
	}
	return r.update(binding, existing)
}

func (r RoleBindingRepo) UpsertWithVersion(binding models.RoleBinding, expectedVersion int64) (models.RoleBinding, error) {
	existing, exists, err := r.GetByUid(binding.EiamUid)
	if err != nil {
		return binding, err
	}
	if !exists {
		return r.create(binding)
	}
	if existing.Version != expectedVersion {
		return binding, ErrRoleBindingConflict
	}
	return r.update(binding, existing)
}

func (r RoleBindingRepo) Delete(eiamUid int64) error {
	res := r.db.Where("eiam_uid = ?", eiamUid).Delete(&models.RoleBinding{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r RoleBindingRepo) create(binding models.RoleBinding) (models.RoleBinding, error) {
	binding.Version = 0 // DDL 默认 0
	binding.CreatedAt = time.Now()
	binding.UpdatedAt = time.Now()
	if err := r.db.Create(&binding).Error; err != nil {
		return binding, err
	}
	return binding, nil
}

func (r RoleBindingRepo) update(binding models.RoleBinding, existing models.RoleBinding) (models.RoleBinding, error) {
	binding.Version = existing.Version + 1
	binding.UpdatedAt = time.Now()
	err := r.db.Model(&models.RoleBinding{}).
		Where("eiam_uid = ?", binding.EiamUid).
		Updates(map[string]interface{}{
			"role":       binding.Role,
			"version":    binding.Version,
			"updated_at": binding.UpdatedAt,
		}).Error
	if err != nil {
		return binding, err
	}
	return binding, nil
}
