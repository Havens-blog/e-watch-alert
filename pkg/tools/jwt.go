package tools

import (
	"errors"
	"time"
	"watchAlert/config"

	"github.com/dgrijalva/jwt-go"
)

// JwtCustomClaims 旧本地 JWT 声明（过渡期仅用于验签，不再签发）
type JwtCustomClaims struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Pass           string `json:"pass"`
	StandardClaims jwt.StandardClaims
}

const (
	// TokenType Token 类型
	TokenType = "bearer"
	// AppGuardName 颁发者（历史遗留，仅验签阶段保留）
	AppGuardName = "WatchAlert"
)

// verifyKey 返回旧本地 JWT 的验签只读副本（仅 Parse/校验，禁 Sign）。
func verifyKey() []byte {
	return []byte(config.Application.Jwt.VerifyOnlyKey)
}

func (j JwtCustomClaims) Valid() error {
	if j.StandardClaims.ExpiresAt > 0 && time.Now().Unix() > j.StandardClaims.ExpiresAt {
		return errors.New("token is expired")
	}
	return nil
}

// ParseToken 解析旧本地 JWT（验签只读副本）。
func ParseToken(tokenStr string) (JwtCustomClaims, error) {
	iJwtCustomClaims := JwtCustomClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, &iJwtCustomClaims, func(token *jwt.Token) (interface{}, error) {
		return verifyKey(), nil
	})

	if err == nil && !token.Valid {
		err = errors.New("invalid Token")
	}
	return iJwtCustomClaims, err
}

func GetUser(tokenStr string) string {
	if tokenStr == "" {
		return ""
	}

	tokenStr = tokenStr[len(TokenType)+1:]
	token, err := ParseToken(tokenStr)
	if err != nil {
		return ""
	}
	return token.Name
}

func GetUserID(tokenStr string) string {
	if tokenStr == "" {
		return ""
	}

	tokenStr = tokenStr[len(TokenType)+1:]
	token, err := ParseToken(tokenStr)
	if err != nil {
		return ""
	}

	return token.ID
}
