package ftl

import (
	"encoding/base64"
	"fmt"
	"strings"
	"os"
	"io"
	"crypto/md5"
)

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

func fileHashPrefix(file *os.File) (string, error) {
	defer file.Seek(0, 0)

	h := md5.New()

	_, err := io.Copy(h, file)
	if err != nil {
		fmt.Println("Error copying file", err)
		return "", err
	}

	hashEncode := encodeBytes(h.Sum(nil))
	return hashEncode[:2], nil
}
