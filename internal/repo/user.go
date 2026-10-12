package repo

import (
	"context"
	"errors"
	"fmt"
	"time"
	"watchAlert/internal/models"
	"watchAlert/pkg/client"
	"watchAlert/pkg/tools"

	"github.com/bytedance/sonic"
	"github.com/zeromicro/go-zero/core/logc"
	"gorm.io/gorm"
)

type (
	UserRepo struct {
		entryRepo
	}

	InterUserRepo interface {
		List(query, joinDuty string) ([]models.Member, error)
		Get(userId, username, email, phone string) (models.Member, bool, error)
		GetUserByRole(role string) ([]models.Member, error)
		Create(r models.Member) error
		Update(r models.Member) error
		SyncLocal(users []models.Member) error
		Delete(userId string) error
		ChangeCache(userId string)
		ChangePass(userId, password string) error
	}
)

func newUserInterface(db *gorm.DB, g InterGormDBCli) InterUserRepo {
	return &UserRepo{
		entryRepo{
			g:  g,
			db: db,
		},
	}
}

func (ur UserRepo) List(query, joinDuty string) ([]models.Member, error) {
	var (
		data []models.Member
		db   = ur.db.Model(&models.Member{})
	)

	if query != "" {
		db.Where("user_name LIKE ? OR email Like ? OR phone LIKE ?", "%"+query+"%", "%"+query+"%", "%"+query+"%")
	}
	if joinDuty == "true" {
		db.Where("join_duty = ?", "true")
	}
	err := db.Find(&data).Error
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (ur UserRepo) Get(userId, username, email, phone string) (models.Member, bool, error) {
	var (
		data models.Member
		db   = ur.db.Model(models.Member{})
	)

	if userId != "" {
		db.Where("user_id = ?", userId)
	}
	if username != "" {
		db.Where("user_name = ?", username)
	}
	if email != "" {
		db.Where("email = ?", email)
	}
	if phone != "" {
		db.Where("phone = ?", phone)
	}

	err := db.First(&data).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return data, false, fmt.Errorf("用户不存在")
		}
		return data, false, err
	}

	return data, true, nil
}

func (ur UserRepo) GetUserByRole(role string) ([]models.Member, error) {
	var (
		data []models.Member
		db   = ur.db.Model(&models.Member{})
	)

	db.Where("role = ?", role)

	err := db.Find(&data).Error
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (ur UserRepo) Create(r models.Member) error {
	err := ur.g.Create(models.Member{}, r)
	if err != nil {
		return err
	}

	if r.UserId == "1" {
		// 人员用户 id 已统一为 eiam uid：admin 即 eiam uid "1"，默认归属嘉立创租户（jlc id=3）
		r.Tenants = append(r.Tenants, "3")
		err = ur.g.Updates(Updates{
			Table: models.Member{},
			Where: map[string]interface{}{
				"user_id = ?": r.UserId,
			},
			Updates: r,
		})
	}

	return nil
}

func (ur UserRepo) Update(r models.Member) error {
	u := Updates{
		Table: models.Member{},
		Where: map[string]interface{}{
			"user_id = ?": r.UserId,
		},
		Updates: r,
	}

	err := ur.g.Updates(u)
	if err != nil {
		return err
	}

	return nil
}

// SyncLocal 确保 eiam 用户在本地 members 表存在扩展记录（按 eiam uid 关联）。
// 本地表只承载应用侧扩展属性（join_duty 等），身份信息以 eiam 为准。
// JoinDuty 为空时保持原值（仅确保记录存在）；"true"/"false" 时显式更新开关。
func (ur UserRepo) SyncLocal(users []models.Member) error {
	for _, u := range users {
		if u.UserId == "" {
			continue
		}

		var exist models.Member
		err := ur.db.Where("user_id = ?", u.UserId).First(&exist).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			row := models.Member{
				UserId:   u.UserId,
				UserName: u.UserName,
				Role:     "app",
				JoinDuty: u.JoinDuty,
				CreateBy: "eiam",
				CreateAt: time.Now().Unix(),
			}
			if err = ur.db.Create(&row).Error; err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			updates := map[string]interface{}{"user_name": u.UserName}
			if u.JoinDuty != "" {
				updates["join_duty"] = u.JoinDuty
			}
			if err = ur.db.Model(&models.Member{}).
				Where("user_id = ?", u.UserId).
				Updates(updates).Error; err != nil {
				return err
			}
		}

		ur.ChangeCache(u.UserId)
	}

	return nil
}

func (ur UserRepo) Delete(userId string) error {
	userInfo, _, err := ur.User().Get(userId, "", "", "")
	if err != nil {
		return err
	}

	for _, tid := range userInfo.Tenants {
		err = ur.Tenant().RemoveTenantLinkedUsers(tid, userId)
		if err != nil {
			return err
		}
	}

	d := Delete{
		Table: models.Member{},
		Where: map[string]interface{}{
			"user_id = ?": userId,
		},
	}
	err = ur.g.Delete(d)
	if err != nil {
		return err
	}

	return nil
}

func (ur UserRepo) ChangeCache(userId string) {
	var dbUser models.Member
	ur.db.Model(&models.Member{}).Where("user_id = ?", userId).First(&dbUser)

	var cacheUser models.Member
	result, err := client.Redis.Get("uid-" + userId).Result()
	if err != nil {
		logc.Error(context.Background(), err)
	}
	_ = sonic.Unmarshal([]byte(result), &cacheUser)

	duration, _ := client.Redis.TTL("uid-" + userId).Result()
	client.Redis.Set("uid-"+userId, tools.JsonMarshalToString(dbUser), duration)
}

func (ur UserRepo) ChangePass(userId, password string) error {
	u := Update{
		Table: models.Member{},
		Where: map[string]interface{}{
			"user_id = ?": userId,
		},
		Update: []string{"password", password},
	}

	err := ur.g.Update(u)
	if err != nil {
		return err
	}

	return nil
}
