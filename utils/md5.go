package utils

import (
	"crypto/md5"
	"encoding/hex"
)

// Md5 计算字符串的 MD5 值，返回 32 位小写十六进制字符串
func Md5(text string) string {
	h := md5.New()
	h.Write([]byte(text))
	return hex.EncodeToString(h.Sum(nil))
}
