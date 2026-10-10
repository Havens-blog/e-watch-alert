package config

import (
	"log"

	"github.com/spf13/viper"
)

type App struct {
	Server   Server   `json:"Server"`
	Database Database `json:"Database"`
	Redis    Redis    `json:"Redis"`
	Jwt      Jwt      `json:"Jwt"`
	Session  Session  `json:"Session"`
	Probe    Probe    `json:"Probe"`
	Jaeger   Jaeger   `json:"Jaeger"`
}

type Server struct {
	Mode           string `json:"mode"`
	Port           string `json:"port"`
	EnableElection bool   `json:"enableElection"`
}

type Database struct {
	Type    string `json:"type"`    // mysql 或 sqlite
	Host    string `json:"host"`    // MySQL 主机地址
	Port    string `json:"port"`    // MySQL 端口
	User    string `json:"user"`    // MySQL 用户名
	Pass    string `json:"pass"`    // MySQL 密码
	DBName  string `json:"dbName"`  // MySQL 数据库名
	Timeout string `json:"timeout"` // MySQL 连接超时
	Path    string `json:"path"`    // SQLite 数据库文件路径
}

type Redis struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Pass     string `json:"pass"`
	Database int    `json:"database"`
}

type Jwt struct {
	// VerifyOnlyKey 旧本地 JWT 的验签只读副本（过渡期读接口容忍旧 token，禁 Sign；可为空=全新部署无旧 token）
	VerifyOnlyKey string `json:"verify_only_key"`
}

// Session 共享会话配置（复用 eiam 同款 ginx/session，键/cookie 须与 eiam 一致）
type Session struct {
	SessionEncryptedKey string        `json:"session_encrypted_key" mapstructure:"session_encrypted_key"` // 共享会话 HS256 签名密钥，与 eiam 一致
	TokenCarrier        string        `json:"token_carrier" mapstructure:"token_carrier"`                 // cookie | token（默认 cookie）
	Cookie              SessionCookie `json:"cookie" mapstructure:"cookie"`
}

type SessionCookie struct {
	Domain   string `json:"domain" mapstructure:"domain"`
	Name     string `json:"name" mapstructure:"name"`
	Secure   *bool  `json:"secure" mapstructure:"secure"`
	HttpOnly *bool  `json:"http_only" mapstructure:"http_only"`
}

type Jaeger struct {
	URL string `json:"url"`
}

// Probe 降级探针配置（tech-design §2.3：探针 A=eiam /health；探针 B=Redis PING）。
type Probe struct {
	EiamHealthURL string `json:"eiam_health_url"` // 探针 A 目标：eiam GET /health 完整 URL（空=禁用探针 A）
	IntervalSec   int    `json:"interval_sec"`    // 探测间隔（秒），默认 10
	TimeoutSec    int    `json:"timeout_sec"`     // 探测超时（秒），默认 3
	Hysteresis    int    `json:"hysteresis"`      // 连续翻转阈值（3 失败→degraded、3 成功→recovered），默认 3
}

var (
	Application App
	Version     string
	configFile  = "config/config.yaml"
)

func InitConfig(version string) {
	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		log.Fatal("配置读取失败:", err)
	}
	var config App
	if err := v.Unmarshal(&config); err != nil {
		log.Fatal("配置解析失败:", err)
	}

	Version = version
	Application = config
}
