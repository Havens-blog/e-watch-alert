package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ApiError 遵循 TECH-error-001：service 层返回 ApiError（kind/code/httpStatus/msg），
// controller/middleware 统一写 HTTP 状态 + TECH-api-001 信封 {code,msg,data}。
// ErrorKind：unauthorized | forbidden | validation | conflict | unavailable | unknown。
type ApiError struct {
	Kind       string
	Code       int
	HTTPStatus int
	Msg        string
}

func (e *ApiError) Error() string { return e.Msg }

func NewValidationError(msg string) *ApiError {
	return &ApiError{Kind: "validation", Code: 1400, HTTPStatus: http.StatusBadRequest, Msg: msg}
}

func NewConflictError(msg string) *ApiError {
	return &ApiError{Kind: "conflict", Code: 1409, HTTPStatus: http.StatusConflict, Msg: msg}
}

func NewNotFoundError(msg string) *ApiError {
	return &ApiError{Kind: "unknown", Code: 1404, HTTPStatus: http.StatusNotFound, Msg: msg}
}

func NewUnauthorizedError(msg string) *ApiError {
	return &ApiError{Kind: "unauthorized", Code: 1401, HTTPStatus: http.StatusUnauthorized, Msg: msg}
}

func NewForbiddenError(msg string) *ApiError {
	return &ApiError{Kind: "forbidden", Code: 1403, HTTPStatus: http.StatusForbidden, Msg: msg}
}

func NewUnavailableError(msg string) *ApiError {
	return &ApiError{Kind: "unavailable", Code: 1503, HTTPStatus: http.StatusServiceUnavailable, Msg: msg}
}

func NewInternalError(msg string) *ApiError {
	return &ApiError{Kind: "unknown", Code: 1500, HTTPStatus: http.StatusInternalServerError, Msg: msg}
}

// Envelope 写 TECH-api-001 信封。
// err == nil → 200 {code:0,msg:"ok",data}；*ApiError → 对应业务码 + HTTP 状态；其余 → 500 {code:1500}。
func Envelope(c *gin.Context, data interface{}, err error) {
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": data})
		return
	}
	if apiErr, ok := err.(*ApiError); ok {
		c.JSON(apiErr.HTTPStatus, gin.H{"code": apiErr.Code, "msg": apiErr.Msg, "data": nil})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"code": 1500, "msg": err.Error(), "data": nil})
}
