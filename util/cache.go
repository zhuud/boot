// Package utils 提供无状态的字符串、切片、时间与缓存键工具函数。
package util

import (
	"fmt"
	"math/rand/v2"
	"time"
)

// GetCacheKey 按 fmt.Sprintf 规则格式化缓存键。
func GetCacheKey(key string, args ...any) string {
	return fmt.Sprintf(key, args...)
}

// GetCacheExpWithRand 返回 t 加上 [0, t/2) 的随机抖动，减轻缓存同时失效。t/2<=0 时返回 t。
func GetCacheExpWithRand(t time.Duration) time.Duration {
	jitter := t / 2
	if jitter <= 0 {
		return t
	}
	return t + time.Duration(rand.Int64N(int64(jitter)))
}
