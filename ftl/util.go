package ftl

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
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

func NewRevisionInfo(revisionName string) RevisionInfo {
	parts := strings.Split(revisionName, ".")
	if len(parts) < 2 {
		fmt.Println("Failed to parse revision", revisionName)
		return RevisionInfo{"", ""}
	}

	return RevisionInfo{parts[0], parts[1]}
}

func NewRevisionInfoFromFile(filePath string) (revision RevisionInfo, err error) {
	name := filepath.Base(filePath)
	parts := strings.Split(name, ".")
	packageName := parts[0]

	file, err := os.Open(filePath)
	if err != nil {
		return
	}

	id, err := buildRevisionId(file)
	if err != nil {
		return
	}

	revision = RevisionInfo{PackageName: packageName, Revision: id}
	return
}

func (ri *RevisionInfo) Name() string {
	return fmt.Sprintf("%v.%v", ri.PackageName, ri.Revision)
}

func (ri *RevisionInfo) Equal(ori RevisionInfo) bool {
	return ri.Revision == ori.Revision
}

func (ri *RevisionInfo) Valid() bool {
	return ri.PackageName != "" && ri.Revision != ""
}

type RevisionListResult struct {
	Revisions []RevisionInfo
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

func buildRevisionId(file *os.File) (revisionId string, err error) {
	// Revsion id will be based on a combination of encoding timestamp and sha1 of the file.
	hashPrefix, err := fileHashPrefix(file)
	if err != nil {
		return
	}

	now := time.Now().UTC()
	hour, min, sec := now.Clock()
	timeStamp := fmt.Sprintf("%s%05d", now.Format("20060102"), hour*60*60+min*60+sec)

	// We're using pieces of our encoding data:
	//  * for our timestamp, we're stripping off all but one of the heading zeros which is encoded as a dash. Also, the last = (buffer)
	//  * For our hash, we're only using 2 bytes
	revisionId = fmt.Sprintf("%s%s", timeStamp, hashPrefix)
	return
}
