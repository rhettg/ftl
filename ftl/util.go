package ftl

import "encoding/base64"

func encodeBytes(b []byte) (s string) {
	enc := base64.NewEncoding("-0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz")
	s = enc.EncodeToString(b)
	return
}

