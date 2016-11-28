package ftl

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type RemoteRepository struct {
	svc        *s3.S3
	bucketName string
}

func NewRemoteRepository(bucketName string, sess *session.Session) (remote *RemoteRepository) {
	svc := s3.New(sess)
	return &RemoteRepository{svc, bucketName}
}

func (rr *RemoteRepository) ListRevisions(packageName string) (revisionList []RevisionInfo, err error) {
	err = rr.svc.ListObjectsPages(
		&s3.ListObjectsInput{
			Bucket:    aws.String(rr.bucketName),
			Prefix:    aws.String(packageName + "."),
			Delimiter: aws.String("."),
		},
		func(p *s3.ListObjectsOutput, lastPage bool) bool {
			for _, cp := range p.CommonPrefixes {
				prefix := aws.StringValue(cp.Prefix)
				revisionName := prefix[:len(prefix)-1]
				revision := NewRevisionInfo(revisionName)
				revisionList = append(revisionList, revision)
			}

			return true
		})

	if err != nil {
		fmt.Println("Failed listing", err)
		return
	}

	return
}

func (rr *RemoteRepository) ListPackages() (pkgs []string, err error) {
	err = rr.svc.ListObjectsPages(
		&s3.ListObjectsInput{
			Bucket:    aws.String(rr.bucketName),
			Prefix:    aws.String(""),
			Delimiter: aws.String("."),
		},
		func(p *s3.ListObjectsOutput, lastPage bool) bool {
			for _, cp := range p.CommonPrefixes {
				prefix := aws.StringValue(cp.Prefix)
				pkgs = append(pkgs, prefix[:len(prefix)-1])
			}

			return true
		})

	if err != nil {
		err = fmt.Errorf("Failed listing: %v", err)
		return
	}

	return
}

func (rr *RemoteRepository) GetRevision(revision RevisionInfo) (filePath string, err error) {
	listResp, err := rr.svc.ListObjects(
		&s3.ListObjectsInput{
			Bucket:    aws.String(rr.bucketName),
			Prefix:    aws.String(revision.Name()),
			Delimiter: aws.String(""),
		})

	if err != nil {
		fmt.Println("Failed listing", err)
		return
	}

	if len(listResp.Contents) > 0 {
		var tmpDir string
		tmpDir, err = ioutil.TempDir("", revision.Name())
		if err != nil {
			return
		}

		filePath = filepath.Join(tmpDir, aws.StringValue(listResp.Contents[0].Key))

		var tmpFile *os.File
		tmpFile, err = os.Create(filePath)
		defer tmpFile.Close()

		downloader := s3manager.NewDownloaderWithClient(rr.svc)
		_, err = downloader.Download(tmpFile,
			&s3.GetObjectInput{
				Bucket: aws.String(rr.bucketName),
				Key:    listResp.Contents[0].Key})

		if err != nil {
			return
		}
	}

	return
}

func (rr *RemoteRepository) Add(ri RevisionInfo, filePath string) (err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("Failed to open file: %s", err.Error())
	}

	statInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("Error stat'ing file: %s", err.Error())
	}

	fileName := filepath.Base(filePath)
	nameBase := fileName[:strings.Index(fileName, ".")]

	// TODO: Use an uploader now that we have a file
	s3Path := fmt.Sprintf("%s.%s.%s", nameBase, ri.Revision, fileName[strings.Index(fileName, ".")+1:])
	_, err = rr.svc.PutObject(&s3.PutObjectInput{
		ACL:           aws.String("private"),
		ContentType:   aws.String("application/octet-stream"),
		ContentLength: aws.Int64(statInfo.Size()),
		Bucket:        aws.String(rr.bucketName),
		Key:           aws.String(s3Path),
		Body:          file,
	})

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
	o, err := rr.svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(rr.bucketName),
		Key:    aws.String(revisionFilePath),
	})

	if err != nil {
		s3Error, _ := err.(awserr.Error)
		if s3Error == nil {
			err = fmt.Errorf("Error retrieving revision, no error")
			return
		} else if s3Error.Code() == "NoSuchKey" {
			err = nil
			return
		} else {
			err = fmt.Errorf("Error finding rev file: %v", err)
			return
		}
	}

	b, err := ioutil.ReadAll(o.Body)
	if err != nil {
		return
	}
	revisionName = string(b)
	return
}

func (rr *RemoteRepository) GetCurrentRevision(packageName string) (revision RevisionInfo, err error) {
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
			err = rr.putRevisionFile(revFile, revisionName)
			if err != nil {
				return
			}

			rr.svc.DeleteObject(&s3.DeleteObjectInput{
				Bucket: aws.String(rr.bucketName),
				Key:    aws.String(oldRevFile),
			})
		}
	}

	if revisionName != "" {
		revision = NewRevisionInfo(revisionName)
	}

	return
}

func (rr *RemoteRepository) putRevisionFile(key string, revision string) (err error) {
	_, err = rr.svc.PutObject(&s3.PutObjectInput{
		ACL:         aws.String("private"),
		ContentType: aws.String("text/plan"),
		Bucket:      aws.String(rr.bucketName),
		Key:         aws.String(key),
		Body:        strings.NewReader(revision),
	})

	if err != nil {
		err = fmt.Errorf("Failed to put new current rev file: %v", err)
		return
	}
	return
}

func (rr *RemoteRepository) GetPreviousRevision(packageName string) (revision RevisionInfo, err error) {
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

func (rr *RemoteRepository) Jump(revision RevisionInfo) error {
	currentRevision, err := rr.GetCurrentRevision(revision.PackageName)
	if err != nil {
		return err
	}

	if currentRevision == revision {
		fmt.Println("Revision is already selected")
		return nil
	}

	if currentRevision.Revision != "" {
		previousFilePath := rr.previousRevisionFilePath(revision.PackageName)
		err = rr.putRevisionFile(previousFilePath, currentRevision.Name())
		if err != nil {
			return err
		}
	}

	currentFilePath := rr.currentRevisionFilePath(revision.PackageName)
	err = rr.putRevisionFile(currentFilePath, revision.Name())
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

	err = rr.putRevisionFile(previousFilePath, currentRevision)
	if err != nil {
		return fmt.Errorf("Failed to put previous rev file: %v", err)
	}

	err = rr.putRevisionFile(currentFilePath, previousRevision)
	if err != nil {
		return fmt.Errorf("Failed to put current rev file: %v", err)
	}

	return nil
}

func (rr *RemoteRepository) PurgeRevision(revision RevisionInfo) (err error) {
	activeRevision, err := rr.GetCurrentRevision(revision.PackageName)
	if err != nil {
		return
	}

	if activeRevision == revision {
		err = errors.New("Can't purge active revision")
		return
	}

	listResp, err := rr.svc.ListObjects(
		&s3.ListObjectsInput{
			Bucket:    aws.String(rr.bucketName),
			Prefix:    aws.String(revision.Name() + "."),
			Delimiter: aws.String("/"),
		})

	if err != nil {
		fmt.Println("Failed listing", err)
		err = fmt.Errorf("Failed listing %v", err)
		return
	}

	if len(listResp.Contents) > 0 {
		_, err := rr.svc.DeleteObject(&s3.DeleteObjectInput{Bucket: aws.String(rr.bucketName), Key: listResp.Contents[0].Key})
		if err != nil {
			fmt.Printf("Failed to remove", err)
			err = fmt.Errorf("Failed to do s3 Del: %v", err)
		}
	} else {
		err = errors.New("Failed to find revision")
	}

	return
}
