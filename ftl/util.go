package ftl

import "encoding/base64"
import "strings"

func encodeBytes(b []byte) (s string) {
	enc := base64.NewEncoding("-0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz")
	s = enc.EncodeToString(b)
	return
}

type RevisionInfo struct {
	PackageName string
	Revision string
}

func NewRevisionInfo(revisionName string) *RevisionInfo {
	parts := strings.Split(revisionName, ".")
	return &RevisionInfo{parts[0], parts[1]}
}

