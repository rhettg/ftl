package ftl

import "fmt"
import "io"
import "launchpad.net/goamz/s3"
import "launchpad.net/goamz/aws"

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
