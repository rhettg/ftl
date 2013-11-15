package ftl

import (
	"errors"
	"fmt"
	"io"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"os"
	"strings"
	"time"
)

func buildRevisionId(file *os.File) (revisionId string, err error) {
	// Revsion id will be based on a combination of encoding timestamp and sha1 of the file.
	hashPrefix, err := fileHashPrefix(file)
	if err != nil {
		return
	}

	now := time.Now().UTC()
	hour, min, sec := now.Clock()
	timeStamp := fmt.Sprintf("%s%d", now.Format("20060102"), hour*60*60+min*60+sec)

	// We're using pieces of our encoding data:
	//  * for our timestamp, we're stripping off all but one of the heading zeros which is encoded as a dash. Also, the last = (buffer)
	//  * For our hash, we're only using 2 bytes
	revisionId = fmt.Sprintf("%s%s", timeStamp, hashPrefix)
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

	listResp, err := rr.bucket.List(packageName+".", ".", "", 1000)
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

func (rr *RemoteRepository) ListPackages() (pkgs []string, err error) {
	pkgs = make([]string, 0, 1000)

	listResp, e := rr.bucket.List("", ".", "", 1000)
	if e != nil {
		err = fmt.Errorf("Failed listing: %v", err)
	}

	for _, prefix := range listResp.CommonPrefixes {
		pkgs = append(pkgs, prefix[:len(prefix)-1])
	}
	return
}

func (rr *RemoteRepository) GetRevisionReader(revisionName string) (fileName string, reader io.ReadCloser, err error) {
	listResp, err := rr.bucket.List(revisionName, "", "", 1)
	if err != nil {
		fmt.Println("Failed listing", err)
		return
	}

	if len(listResp.Contents) > 0 {
		fileName = listResp.Contents[0].Key
		reader, err = rr.bucket.GetReader(fileName)
	}

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

	fileName := statInfo.Name()
	nameBase := fileName[:strings.Index(fileName, ".")]

	s3Path := fmt.Sprintf("%s.%s.%s", nameBase, revisionId, fileName[strings.Index(fileName, ".")+1:])
	rr.bucket.PutReader(s3Path, file, statInfo.Size(), "application/octet-stream", s3.Private)
	return
}

func (rr *RemoteRepository) currentRevisionFilePath(packageName string) (revisionPath string) {
	revisionPath = fmt.Sprintf("%s.rev", packageName)
	return
}

func (rr *RemoteRepository) GetCurrentRevision(packageName string) (revisionName string, err error) {
	revFile := rr.currentRevisionFilePath(packageName)

	data, err := rr.bucket.Get(revFile)
	if err != nil {
		s3Error, _ := err.(*s3.Error)
		if s3Error == nil {
			err = fmt.Errorf("Error retrieving revision, no error")
			return
		} else if s3Error.StatusCode == 404 {
			err = nil
			return
		} else {
			err = fmt.Errorf("Error finding rev file: %v", err)
			return
		}
	}

	revisionName = string(data)
	return
}

func (rr *RemoteRepository) Jump(packageName, revisionName string) error {
	// TODO: Verify revision?
	activeFile := rr.currentRevisionFilePath(packageName)

	err := rr.bucket.Put(activeFile, []byte(revisionName), "text/plain", s3.Private)
	if err != nil {
		err = fmt.Errorf("Failed to put rev file: %v", err)
	}

	return err
}

func (rr *RemoteRepository) PurgeRevision(revisionName string) (err error) {
	pkgName := revisionName[:strings.Index(revisionName, ".")]
	activeRevision, err := rr.GetCurrentRevision(pkgName)
	if err != nil {
		return
	}

	if activeRevision == revisionName {
		err = errors.New("Can't purge active revision")
		return
	}

	listResp, err := rr.bucket.List(revisionName+".", "/", "", 1)
	if err != nil {
		fmt.Println("Failed listing", err)
		err = fmt.Errorf("Failed listing %v", err)
		return
	}

	if len(listResp.Contents) > 0 {
		err = rr.bucket.Del(listResp.Contents[0].Key)
		if err != nil {
			fmt.Printf("Failed to remove", err)
			err = fmt.Errorf("Failed to do s3 Del: %v", err)
		}
	} else {
		err = errors.New("Failed to find revision")
	}

	return
}
