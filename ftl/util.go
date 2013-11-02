package ftl

import "encoding/base64"
import "strings"
import "fmt"

func encodeBytes(b []byte) (s string) {
	enc := base64.NewEncoding("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz~")
	s = enc.EncodeToString(b)
	return
}

type RevisionInfo struct {
	PackageName string
	Revision    string
}

func NewRevisionInfo(revisionName string) *RevisionInfo {
	parts := strings.Split(revisionName, ".")
	if len(parts) < 2 {
		fmt.Println("Failed to parse revision", revisionName)
		return nil
	}

	return &RevisionInfo{parts[0], parts[1]}
}
