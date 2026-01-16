package retry

import (
	"math"
	"net/http"
	"strconv"
	"time"
)

const (
	// InitialDelay 初始延迟
	InitialDelay = 500 * time.Millisecond

	// BackoffFactor 退避因子
	BackoffFactor = 2.0

	// MaxDelayWithHeader 有响应头时的最大延迟
	MaxDelayWithHeader = 10 * time.Second

	// MaxDelayNoHeader 无响应头时的最大延迟
	MaxDelayNoHeader = 2 * time.Second

	// MaxRetries 最大重试次数
	MaxRetries = 3
)

// CalculateDelay 计算重试延迟
func CalculateDelay(attempt int, resp *http.Response) time.Duration {
	// 优先级 1: 使用 HTTP 响应头
	if resp != nil {
		if delay := parseRetryAfter(resp.Header); delay > 0 {
			// 限制最大延迟
			if delay > MaxDelayWithHeader {
				return MaxDelayWithHeader
			}
			return delay
		}
	}

	// 优先级 2: 指数退避
	// delay = InitialDelay * (BackoffFactor ^ (attempt - 1))
	exponent := float64(attempt - 1)
	delay := time.Duration(float64(InitialDelay) * math.Pow(BackoffFactor, exponent))

	// 限制最大延迟
	maxDelay := MaxDelayNoHeader
	if resp != nil {
		maxDelay = MaxDelayWithHeader
	}

	if delay > maxDelay {
		return maxDelay
	}

	return delay
}

// parseRetryAfter 解析 Retry-After 头
func parseRetryAfter(header http.Header) time.Duration {
	// 1. 尝试 Retry-After-Ms（毫秒）
	if ms := header.Get("Retry-After-Ms"); ms != "" {
		if val, err := strconv.ParseInt(ms, 10, 64); err == nil && val > 0 {
			return time.Duration(val) * time.Millisecond
		}
	}

	// 2. 尝试 Retry-After（秒或 HTTP-Date）
	if ra := header.Get("Retry-After"); ra != "" {
		// 尝试解析为秒数
		if seconds, err := strconv.ParseFloat(ra, 64); err == nil && seconds > 0 {
			return time.Duration(seconds * float64(time.Second))
		}

		// 尝试解析为 HTTP-Date
		if t, err := http.ParseTime(ra); err == nil {
			delay := time.Until(t)
			if delay > 0 {
				return delay
			}
		}
	}

	return 0
}

// CalculateBackoff 计算指数退避延迟（不考虑响应头）
func CalculateBackoff(attempt int) time.Duration {
	return CalculateDelay(attempt, nil)
}

// CalculateBackoffWithJitter 计算带抖动的退避延迟
// 抖动可以避免多个客户端同时重试造成的"雷群效应"
func CalculateBackoffWithJitter(attempt int, jitterFactor float64) time.Duration {
	baseDelay := CalculateBackoff(attempt)

	// 添加抖动：delay * (1 ± jitterFactor)
	// 例如 jitterFactor = 0.1 表示 ±10% 的抖动
	if jitterFactor > 0 {
		jitter := float64(baseDelay) * jitterFactor * (2.0*float64(time.Now().UnixNano()%1000)/1000.0 - 1.0)
		return time.Duration(float64(baseDelay) + jitter)
	}

	return baseDelay
}
