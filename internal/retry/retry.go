package retry

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Retrier 重试器
type Retrier struct {
	MaxRetries int
	OnRetry    func(attempt int, err error, delay time.Duration) // 重试回调
}

// NewRetrier 创建新的重试器
func NewRetrier() *Retrier {
	return &Retrier{
		MaxRetries: MaxRetries,
		OnRetry:    nil,
	}
}

// NewRetrierWithCallback 创建带回调的重试器
func NewRetrierWithCallback(onRetry func(attempt int, err error, delay time.Duration)) *Retrier {
	return &Retrier{
		MaxRetries: MaxRetries,
		OnRetry:    onRetry,
	}
}

// Do 执行带重试的操作
func (r *Retrier) Do(ctx context.Context, fn func() (*http.Response, error)) (*http.Response, error) {
	var lastResp *http.Response
	var lastErr error

	for attempt := 1; attempt <= r.MaxRetries; attempt++ {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 执行操作
		resp, err := fn()

		// 成功
		if err == nil && (resp == nil || resp.StatusCode < 400) {
			return resp, nil
		}

		// 记录最后的响应和错误
		lastResp = resp
		lastErr = err

		// 检查是否可重试
		retryable := false
		if err != nil {
			retryable = IsRetryable(err)
		} else if resp != nil {
			retryable = IsRetryableStatus(resp.StatusCode)
		}

		if !retryable {
			return resp, err
		}

		// 最后一次尝试失败
		if attempt == r.MaxRetries {
			return resp, err
		}

		// 计算延迟
		delay := CalculateDelay(attempt, resp)

		// 调用重试回调
		if r.OnRetry != nil {
			r.OnRetry(attempt, err, delay)
		}

		// 等待
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			// 继续下一次尝试
		}
	}

	return lastResp, lastErr
}

// DoWithFunc 执行带重试的操作（泛型版本）
func (r *Retrier) DoWithFunc(ctx context.Context, fn func() error) error {
	var lastErr error

	for attempt := 1; attempt <= r.MaxRetries; attempt++ {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 执行操作
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// 检查是否可重试
		if !IsRetryable(err) {
			return err
		}

		// 最后一次尝试失败
		if attempt == r.MaxRetries {
			return err
		}

		// 计算延迟
		delay := CalculateDelay(attempt, nil)

		// 调用重试回调
		if r.OnRetry != nil {
			r.OnRetry(attempt, err, delay)
		}

		// 等待
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// 继续下一次尝试
		}
	}

	return lastErr
}

// DoWithValue 执行带重试的操作（返回值版本）
func DoWithValue[T any](ctx context.Context, fn func() (T, error), maxRetries int) (T, error) {
	var zero T
	var lastErr error
	var lastValue T

	if maxRetries == 0 {
		maxRetries = MaxRetries
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// 检查上下文
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		// 执行操作
		value, err := fn()
		if err == nil {
			return value, nil
		}

		lastErr = err
		lastValue = value

		// 检查是否可重试
		if !IsRetryable(err) {
			return value, err
		}

		// 最后一次尝试
		if attempt == maxRetries {
			return value, err
		}

		// 延迟
		delay := CalculateBackoff(attempt)
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(delay):
		}
	}

	return lastValue, lastErr
}

// RetryStats 重试统计信息
type RetryStats struct {
	TotalAttempts int
	SuccessCount  int
	FailureCount  int
	TotalDelay    time.Duration
}

// String 返回统计信息的字符串表示
func (s *RetryStats) String() string {
	return fmt.Sprintf("Attempts: %d, Success: %d, Failure: %d, Total Delay: %v",
		s.TotalAttempts, s.SuccessCount, s.FailureCount, s.TotalDelay)
}
