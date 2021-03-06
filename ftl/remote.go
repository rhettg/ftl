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
	timeStamp := fmt.Sprintf("%s%05d", now.Format("20060102"), hour*60*60+min*60+sec)

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

func (rr *RemoteRepository) ListRevisions(packageName string) (revisionList []*RevisionInfo, err error) {
	listResp, err := rr.bucket.List(packageName+".", ".", "", 1000)
	if err != nil {
		fmt.Println("Failed listing", err)
		return
	}

	for _, prefix := range listResp.CommonPrefixes {
		revisionName := prefix[:len(prefix)-1]
		revision := NewRevisionInfo(revisionName)
		revisionList = append(revisionList, revision)
	}

	return
}

func (rr *RemoteRepository) ListPackages() (pkgs []string, err error) {
	listResp, e := rr.bucket.List("", ".", "", 1000)
	if e != nil {
		err = fmt.Errorf("Failed listing: %v", e)
		return
	}

	for _, prefix := range listResp.CommonPrefixes {
		pkgs = append(pkgs, prefix[:len(prefix)-1])
	}
	return
}

func (rr *RemoteRepository) GetRevisionReader(revision *RevisionInfo) (fileName string, reader io.ReadCloser, err error) {
	listResp, err := rr.bucket.List(revision.Name(), "", "", 1)
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

func (rr *RemoteRepository) Spool(packageName string, file *os.File) (revision *RevisionInfo, err error) {
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

	revision = &RevisionInfo{packageName, revisionId}

	fileName := statInfo.Name()
	nameBase := fileName[:strings.Index(fileName, ".")]

	s3Path := fmt.Sprintf("%s.%s.%s", nameBase, revisionId, fileName[strings.Index(fileName, ".")+1:])
	err = rr.bucket.PutReader(s3Path, file, statInfo.Size(), "application/octet-stream", s3.Private)
	if err != nil {
		fmt.Println("Failed to PUT revision:", err)
		return
	}
	return
}

func (rr *RemoteRepository) currentRevisionFilePathOld(packageName string) (revisionPath string) {
	revisionPath = fmt.Sprintf("%s.rev", packageName)
	return
}

func (rr *RemoteRepository) currentRevisionFilePath(packageName string) (revisionPath string) {
	revisionPath = fmt.Sprintf("%s.current", packageName)
	return
}

func (rr *RemoteRepository) previousRevisionFilePath(packageName string) (revisionPath string) {
	revisionPath = fmt.Sprintf("%s.previous", packageName)
	return
}

func (rr *RemoteRepository) revisionFromPath(revisionFilePath string) (revisionName string, err error) {
	data, err := rr.bucket.Get(revisionFilePath)
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

func (rr *RemoteRepository) GetCurrentRevision(packageName string) (revision *RevisionInfo, err error) {
	revFile := rr.currentRevisionFilePath(packageName)
	revisionName, err := rr.revisionFromPath(revFile)
	if err != nil {
		return
	}

	if revisionName == "" {
		oldRevFile := rr.currentRevisionFilePathOld(packageName)
		revisionName, err = rr.revisionFromPath(oldRevFile)
		if err != nil {
			return
		}

		if revisionName == "" {
			// This was the old way to name this file, let's port us to the new way:
			err = rr.bucket.Put(revFile, []byte(revisionName), "text/plain", s3.Private)
			if err != nil {
				err = fmt.Errorf("Failed to put new current rev file: %v", err)
				return
			}

			rr.bucket.Del(oldRevFile)
		}
	}

	if revisionName != "" {
		revision = NewRevisionInfo(revisionName)
	}

	return
}

func (rr *RemoteRepository) GetPreviousRevision(packageName string) (revision *RevisionInfo, err error) {
	revFile := rr.previousRevisionFilePath(packageName)
	revisionName, err := rr.revisionFromPath(revFile)
	if err != nil {
		return
	}

	if revisionName != "" {
		revision = NewRevisionInfo(revisionName)
	}

	return
}

func (rr *RemoteRepository) Jump(revision *RevisionInfo) error {
	currentRevision, err := rr.GetCurrentRevision(revision.PackageName)
	if err != nil {
		return err
	}

	if currentRevision == revision {
		fmt.Println("Revision is already selected")
		return nil
	}

	if currentRevision != nil {
		previousFilePath := rr.previousRevisionFilePath(revision.PackageName)
		err = rr.bucket.Put(previousFilePath, []byte(currentRevision.Name()), "text/plain", s3.Private)
		if err != nil {
			return fmt.Errorf("Failed to put previous rev file: %v", err)
		}
	}

	currentFilePath := rr.currentRevisionFilePath(revision.PackageName)
	err = rr.bucket.Put(currentFilePath, []byte(revision.Name()), "text/plain", s3.Private)
	if err != nil {
		return fmt.Errorf("Failed to put rev file: %v", err)
	}

	return nil
}

func (rr *RemoteRepository) JumpBack(packageName string) error {
	previousFilePath := rr.previousRevisionFilePath(packageName)
	currentFilePath := rr.currentRevisionFilePath(packageName)

	previousRevision, err := rr.revisionFromPath(previousFilePath)
	if err != nil {
		return err
	}

	if previousRevision == "" {
		return fmt.Errorf("Failed to find previous revision")
	}

	currentRevision, err := rr.revisionFromPath(currentFilePath)
	if err != nil {
		return err
	}

	if currentRevision == "" {
		return fmt.Errorf("Failed to find current revision")
	}

	err = rr.bucket.Put(previousFilePath, []byte(currentRevision), "text/plain", s3.Private)
	if err != nil {
		return fmt.Errorf("Failed to put previous rev file: %v", err)
	}

	err = rr.bucket.Put(currentFilePath, []byte(previousRevision), "text/plain", s3.Private)
	if err != nil {
		return fmt.Errorf("Failed to put current rev file: %v", err)
	}

	return nil
}

func (rr *RemoteRepository) PurgeRevision(revision *RevisionInfo) (err error) {
	activeRevision, err := rr.GetCurrentRevision(revision.PackageName)
	if err != nil {
		return
	}

	if activeRevision == revision {
		err = errors.New("Can't purge active revision")
		return
	}

	listResp, err := rr.bucket.List(revision.Name()+".", "/", "", 1)
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
