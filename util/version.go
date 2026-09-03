package util

import (
	"slices"
	"strings"
)

// CompareVersions 按点分数字比较 v1 与 v2，op 支持 = == != < > <= >=。无法识别的 op 返回 false。
// 缺段与末尾 0 视为相等，例如 1 == 1.0 == 1.0.0。
func CompareVersions(v1, op, v2 string) bool {
	result := compareVersions(v1, v2)
	switch op {
	case "=", "==":
		return result == 0
	case "!=":
		return result != 0
	case "<":
		return result < 0
	case ">":
		return result > 0
	case "<=":
		return result <= 0
	case ">=":
		return result >= 0
	default:
		return false
	}
}

// compareVersions 返回 <0 / 0 / >0，表示 v1<v2、相等、v1>v2。
func compareVersions(v1, v2 string) int {
	return slices.Compare(versionNums(v1), versionNums(v2))
}

func versionNums(s string) []int {
	s = strings.ReplaceAll(s, "V", "")
	s = strings.ReplaceAll(s, "v", "")
	s = strings.ReplaceAll(s, "-", ".")
	parts := strings.Split(s, ".")
	out := make([]int, len(parts))
	for i, part := range parts {
		n := 0
		for j := 0; j < len(part); j++ {
			if part[j] < '0' || part[j] > '9' {
				break
			}
			n = n*10 + int(part[j]-'0')
		}
		out[i] = n
	}
	// 去掉末尾 0，使 1 与 1.0.0 比较结果相同。
	for len(out) > 0 && out[len(out)-1] == 0 {
		out = out[:len(out)-1]
	}
	return out
}
