package comm

import (
	"crypto/md5"
	"encoding/hex"
)

func Md5hash(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}
