package retry

import (
	"errors"
	"net/http"
	"strings"
)

// ErrorType 错误类型
type ErrorType string

const (
	ErrorTypeRetryable    ErrorType = "retryable"
	ErrorTypeNonRetryable ErrorType = "non_retryable"
)

// ClassifyError 分类错误
func ClassifyError(err error) ErrorType {
	if err == nil {
		return ErrorTypeNonRetryable
	}

	errMsg := err.Error()
	errMsgLower := strings.ToLower(errMsg)

	// 1. 网络错误（可重试）
	networkErrors := []string{
		"connection reset",
		"connection refused",
		"timeout",
		"temporary failure",
		"econnreset",
		"etimedout",
		"eof",
		"broken pipe",
		"no route to host",
	}
	for _, ne := range networkErrors {
		if strings.Contains(errMsgLower, ne) {
			return ErrorTypeRetryable
		}
	}

	// 2. API 错误码（可重试）
	retryableMessages := []string{
		"overloaded",
		"exhausted",
		"too many requests",
		"rate limit",
		"503",
		"502",
		"504",
		"429",
		"service unavailable",
		"bad gateway",
		"gateway timeout",
	}
	for _, rm := range retryableMessages {
		if strings.Contains(errMsgLower, rm) {
			return ErrorTypeRetryable
		}
	}

	// 3. 其他错误（不可重试）
	return ErrorTypeNonRetryable
}

// IsRetryable 检查错误是否可重试
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	return ClassifyError(err) == ErrorTypeRetryable
}

// ClassifyHTTPStatus 根据 HTTP 状态码分类
func ClassifyHTTPStatus(status int) ErrorType {
	// 2xx 成功
	if status >= 200 && status < 300 {
		return ErrorTypeNonRetryable
	}

	// 4xx 客户端错误（通常不可重试）
	if status >= 400 && status < 500 {
		// 特殊情况：429 Too Many Requests 可重试
		if status == http.StatusTooManyRequests {
			return ErrorTypeRetryable
		}
		// 408 Request Timeout 可重试
		if status == http.StatusRequestTimeout {
			return ErrorTypeRetryable
		}
		return ErrorTypeNonRetryable
	}

	// 5xx 服务器错误（可重试）
	if status >= 500 && status < 600 {
		return ErrorTypeRetryable
	}

	return ErrorTypeNonRetryable
}

// IsRetryableStatus 检查 HTTP 状态码是否可重试
func IsRetryableStatus(status int) bool {
	return ClassifyHTTPStatus(status) == ErrorTypeRetryable
}

// WrapError 包装错误并附加元数据
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return errors.New(message + ": " + err.Error())
}
