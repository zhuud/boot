package util

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"hash/crc32"
	"io"
)

// crypto/rand、math/rand/v2（别名 cryptorand / mathrand；v2 不是更安全）
//
//	方法                                作用                                  用法
//	crypto/rand.Text                    生成会话 token，26 位 base32          cryptorand.Text()
//	crypto/rand.Read                    用随机字节填满 buf；失败则进程崩溃    cryptorand.Read(buf)
//	crypto/rand.Int                     生成 [0, max) 的均匀大整数            cryptorand.Int(cryptorand.Reader, max)
//	math/rand/v2.IntN                   随机整数 [0, n)，用来抽下标           mathrand.IntN(len(items))
//	math/rand/v2.N                      IntN 的泛型版，任意整数类型           mathrand.N(int64(n))
//	math/rand/v2.Shuffle                原地打乱切片                          mathrand.Shuffle(len(s), swap)
//	math/rand/v2.Perm                   返回 0..n-1 的随机排列                mathrand.Perm(n)
//	math/rand/v2.New(NewPCG)            固定种子便于单测；*Rand 不能并发      r := mathrand.New(mathrand.NewPCG(a, b))

const (
	letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	digitBytes  = "0123456789"
)

// Md5 返回 s 的 MD5 十六进制摘要，仅用于非安全校验。
func Md5(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// HmacSha256 用 key 对 value 做 HMAC-SHA256，返回 hex。key 是密钥不是 salt。
func HmacSha256(value, key string) string {
	return hex.EncodeToString(hmacSha256(value, key))
}

// HmacSha256Verify 校验 signature 是否为 value 在 key 下的 HMAC-SHA256 hex。
// 用 hmac.Equal 常时比较；hex 非法时返回 false。
func HmacSha256Verify(value, key, signature string) bool {
	want, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	return hmac.Equal(hmacSha256(value, key), want)
}

func hmacSha256(value, key string) []byte {
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = io.WriteString(mac, value)
	return mac.Sum(nil)
}

// Crc32 返回 s 的 CRC-32 IEEE 校验和。
func Crc32(s string) uint32 {
	return crc32.ChecksumIEEE([]byte(s))
}

// RandomString 用 crypto/rand 生成长度为 n 的字母数字串。n<=0 时返回空串。
func RandomString(n int) string {
	return randomString(n, letterBytes)
}

// RandomDigits 用 crypto/rand 生成长度为 n 的数字串。n<=0 时返回空串。
func RandomDigits(n int) string {
	return randomString(n, digitBytes)
}

func randomString(n int, alphabet string) string {
	if n <= 0 || alphabet == "" {
		return ""
	}
	alphaLen := len(alphabet)
	// 从 alphabet 里均匀抽出 n 个字符。熵来自 rand.Read，每个字节是 0–255，共 256 种
	// 不能直接 v % 字符集长度  例如 数字 10 种，256 = 25×10 + 6。余数 0–5 会比 6–9 多出现一次，验证码会偏小
	// 直接去掉不能整除的余数，保证每个字符出现次数相等。
	limit := 256 - (256 % alphaLen)
	out := make([]byte, n)
	buf := make([]byte, n*256/limit+16)
	filled := 0
	for filled < n {
		_, _ = rand.Read(buf)
		for _, v := range buf {
			// v 0-255
			if int(v) >= limit {
				continue
			}
			out[filled] = alphabet[int(v)%alphaLen]
			filled++
			if filled == n {
				break
			}
		}
	}
	return string(out)
}
