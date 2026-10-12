package types

type RequestUserCreate struct {
	UserId     string   `json:"userid"`
	UserName   string   `json:"username"`
	Email      string   `json:"email"`
	Phone      string   `json:"phone"`
	Password   string   `json:"password"`
	Role       string   `json:"role"`
	CreateBy   string   `json:"create_by"`
	CreateAt   int64    `json:"create_at"`
	JoinDuty   string   `json:"joinDuty" `
	DutyUserId string   `json:"dutyUserId"`
	Tenants    []string `json:"tenants" gorm:"tenants;serializer:json"`
}

type RequestUserUpdate struct {
	UserId     string   `json:"userid"`
	UserName   string   `json:"username"`
	Email      string   `json:"email"`
	Phone      string   `json:"phone"`
	Password   string   `json:"password"`
	Role       string   `json:"role"`
	CreateBy   string   `json:"create_by"`
	CreateAt   int64    `json:"create_at"`
	JoinDuty   string   `json:"joinDuty" `
	DutyUserId string   `json:"dutyUserId"`
	Tenants    []string `json:"tenants" gorm:"tenants;serializer:json"`
}

type RequestUserQuery struct {
	UserId   string `json:"userid" form:"userid"`
	UserName string `json:"username" form:"username"`
	Email    string `json:"email" form:"email"`
	Phone    string `json:"phone" form:"phone"`
	Query    string `json:"query" form:"query"`
	JoinDuty string `json:"joinDuty" form:"joinDuty"`
}

type RequestUserChangePassword struct {
	UserId   string `json:"userid"`
	Password string `json:"password"`
}

// SyncLocalUser 本地用户扩展同步项（按 eiam uid 关联）
type SyncLocalUser struct {
	UserId   string `json:"userid"`
	UserName string `json:"username"`
	// JoinDuty 为空表示保持本地原值（仅确保本地扩展记录存在）；
	// "true"/"false" 表示显式设置「接受值班」开关。
	JoinDuty string `json:"joinDuty"`
}

// RequestUserSyncLocal 同步 eiam 用户到 WatchAlert 本地扩展表。
// 人员身份以 eiam 为唯一来源，本地 members 表仅承载应用侧扩展属性（如 join_duty）。
type RequestUserSyncLocal struct {
	Users []SyncLocalUser `json:"users"`
}
