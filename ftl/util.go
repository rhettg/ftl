package ftl

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
)

func encodeBytes(b []byte) (s string) {
	// Note that this encoding is not decodable, as we are using '0' for two different bytes.
	// This is much safer for using these as parts of file names.
	enc := base64.NewEncoding("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz0")
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

func (ri *RevisionInfo) Name() string {
	return fmt.Sprintf("%v.%v", ri.PackageName, ri.Revision)
}

type RevisionListResult struct {
	Revisions []*RevisionInfo
	Err       error
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
