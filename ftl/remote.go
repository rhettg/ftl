package ftl

import "fmt"
import "io"
import "os"
import "time"
import "strings"
import "crypto/md5"
import "encoding/binary"
import "launchpad.net/goamz/s3"
import "launchpad.net/goamz/aws"


func buildRevisionId(file *os.File) (revisionId string, err error) {
	// Revsion id will be based on a combination of encoding timestamp and sha1 of the file.
	
	defer file.Seek(0, 0)
	
	h := md5.New()
	
	_, err = io.Copy(h, file)
	if err != nil {
		fmt.Println("Error copying file", err)
		return
	}
	hashEncode := encodeBytes(h.Sum(nil))
	
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(time.Now().Unix()))
	timeStampEncode := encodeBytes(buf)
	
	// We're using pieces of our encoding data:
	//  * for our timestamp, we're stripping off all but one of the heading zeros which is encoded as a dash. Also, the last = (buffer)
	//  * For our hash, we're only using 2 bytes
	revisionId = fmt.Sprintf("%s%s", timeStampEncode[4:len(timeStampEncode)-1], hashEncode[:2])
	return
}

type RemoteRepository struct {
	bucket *s3.Bucket
}

func NewRemoteRepository(name string, auth aws.Auth, region aws.Region) (remote *RemoteRepository) {
	myS3 := s3.New(auth, region)
	bucket := myS3.Bucket(name)
	return &RemoteRepository{bucket}
}

func (rr *RemoteRepository) ListRevisions(packageName string) (revisionList []string) {
	revisionList = make([]string, 0, 1000)
	
	listResp, err := rr.bucket.List(packageName + ".", ".", "", 1000)
	if err != nil {
		fmt.Println("Failed listing", err)
		return 
	}
	
	for _, prefix := range listResp.CommonPrefixes {
		revisionName := prefix[:len(prefix)-1]
		revisionList = append(revisionList, revisionName)
	}
	
	return
}

func (rr *RemoteRepository) ListPackages() (pkgs []string) {
	pkgs = make([]string, 0, 1000)
	
	listResp, err := rr.bucket.List("", ".", "", 1000)
	if err != nil {
		fmt.Println("Failed listing", err)
		return
	}
	
	for _, prefix := range listResp.CommonPrefixes {
		pkgs = append(pkgs, prefix[:len(prefix)-1])
	}
	return
}

func (rr *RemoteRepository) GetRevisionReader(packageName, revisionName string) (reader io.ReadCloser, err error) {
	return
}

func (rr *RemoteRepository) GetBlessedRevision(packageName string) (revisionName string) {
	revFile := fmt.Sprintf("%s.rev", packageName)
	
	data, err := rr.bucket.Get(revFile)
	if err != nil {
		s3Error, _ := err.(*s3.Error)
		if s3Error.StatusCode == 404 {
			return
		} else {
			fmt.Printf("Error finding rev file", err)
			return
		}
	}
	
	revisionName = string(data)
	return
}

func (rr *RemoteRepository) Spool(packageName string, file *os.File) (revisionName string, err error) {
	statInfo, err := file.Stat()
	if err != nil {
		fmt.Println("Error stating file", err)
		return
	}

	revisionId, err := buildRevisionId(file)
	if err != nil {
		fmt.Println("Failed to build revision id")
		return
	}
	
	revisionName = fmt.Sprintf("%s.%s", packageName, revisionId)
	
	parts := strings.Split(statInfo.Name(), ".")
	nameBase := parts[0]
	ext := parts[1]
	
	s3Path := fmt.Sprintf("%s.%s.%s", nameBase, revisionId, ext)
	rr.bucket.PutReader(s3Path, file, statInfo.Size(), "application/octet-stream", s3.Private)
	return
}