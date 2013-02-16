package main

import "fmt"
import "os"
import "io"
import "time"
import "encoding/binary"
import "encoding/base64"
import "path/filepath"
import "crypto/md5"
import "strings"
import goopt "github.com/droundy/goopt"
import "launchpad.net/goamz/s3"
import "launchpad.net/goamz/aws"

var amVerbose = goopt.Flag([]string{"-v", "--verbose"}, []string{"--quiet"},
	"output verbosely", "be quiet, instead")

func optFail(message string) {
		fmt.Println(message)
		fmt.Print(goopt.Help())
		os.Exit(1)
}

func encodeBytes(b []byte) (s string) {
	enc := base64.NewEncoding("-0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz")
	s = enc.EncodeToString(b)
	return
}

func buildRevisionId(fileName string) (string, error) {
	// Revsion id will be based on a combination of encoding timestamp and sha1 of the file.
	
	h := md5.New()
	
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Println("Error opening file", err)
		return "", err
	}

	_, err = io.Copy(h, file)
	if err != nil {
		fmt.Println("Error copying file", err)
		return "", err
	}
	
	hashEncode := encodeBytes(h.Sum(nil))
	
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(time.Now().Unix()))
	timeStampEncode := encodeBytes(buf)
	
	// We're using pieces of our encoding data:
	//  * for our timestamp, we're stripping off all but one of the heading zeros which is encoded as a dash. Also, the last = (buffer)
	//  * For our hash, we're only using 2 bytes
	return fmt.Sprintf("%s%s", timeStampEncode[4:len(timeStampEncode)-1], hashEncode[:2]), nil
}

func blessedRevision(bucket *s3.Bucket, packageName string) string {
	blessFile := fmt.Sprintf("%s.bless", packageName)
	
	data, err := bucket.Get(blessFile)
	if err != nil {
		s3Error, _ := err.(*s3.Error)
		if s3Error.StatusCode == 404 {
			return ""
		} else {
			fmt.Printf("Error finding bless file", err)
			return ""
		}
	}
	
	return string(data)
}

func spoolCmd(bucket *s3.Bucket, fileName string) {
	revisionId, err := buildRevisionId(fileName)
	if err != nil {
		fmt.Println("Failed to build revision id")
		return
	}
	
	name := filepath.Base(fileName)
	parts := strings.Split(name, ".")
	nameBase := parts[0]
	ext := parts[1]
	
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Println("Error opening file", err)
		return
	}
	
	statInfo, err := file.Stat()
	if err != nil {
		fmt.Println("Error stating file", err)
		return
	}
	
	defer file.Close()
	
	s3Path := fmt.Sprintf("%s.%s.%s", nameBase, revisionId, ext)
	bucket.PutReader(s3Path, file, statInfo.Size(), "application/octet-stream", s3.Private)
	
	fmt.Printf("%s.%s\n", nameBase, revisionId)

}

func syncCmd() {
	fmt.Println("Sync")
}

func blessCmd(bucket *s3.Bucket, blessName string) {
	blessParts := strings.Split(blessName, ".")
	packageName := blessParts[0]
	revisionName := blessParts[1]
	
	blessFile := fmt.Sprintf("%s.bless", packageName)
	
	err := bucket.Put(blessFile, []byte(revisionName), "text/plain", s3.Private)
	if err != nil {
		fmt.Printf("Failed to put bless file", err)
		}

}

func listCmd(bucket *s3.Bucket, packageName string) {
	blessed := blessedRevision(bucket, packageName)
	
	listResp, err := bucket.List(packageName + ".", ".", "", 1000)
	if err != nil {
		fmt.Println("Failed listing", err)
		return
	}
	
	for _, prefix := range listResp.CommonPrefixes {
		revisionName := prefix[:len(prefix)-1]
		if strings.HasSuffix(revisionName, blessed) {
			fmt.Printf("%s\t(blessed)\n", revisionName)
		} else {
			fmt.Println(revisionName)
		}
	}
}

func listPackagesCmd(bucket *s3.Bucket) {
	listResp, err := bucket.List("", ".", "", 1000)
	if err != nil {
		fmt.Println("Failed listing", err)
		return
	}
	
	for _, prefix := range listResp.CommonPrefixes {
		fmt.Println(prefix[:len(prefix)-1])
	}
}

func main() {
	goopt.Description = func() string {
		return "Faster Than Light Deploy System"
	}
	goopt.Version = "0.1"
	goopt.Summary = "Deploy system built around S3."
	goopt.Parse(nil)
	
	auth, err := aws.EnvAuth()
    if err != nil {
		optFail(fmt.Sprintf("AWS error: %s", err))
    }
	
	myS3 := s3.New(auth, aws.USEast)
	
	bucket := myS3.Bucket("ftl-rhettg")

	if len(goopt.Args) > 0 {
		cmd := strings.TrimSpace(goopt.Args[0])
		switch cmd {
			case "spool":
				if (len(goopt.Args) > 1) {
					fileName := strings.TrimSpace(goopt.Args[1])
					fullPath, err := filepath.Abs(fileName)
					if err != nil {
						optFail("Unable to parse path")
					}
					
					spoolCmd(bucket, fullPath)
				} else {
					optFail("Missing file name")
				}
			case "bless":
				if (len(goopt.Args) > 1) {
					blessName := strings.TrimSpace(goopt.Args[1])
					blessCmd(bucket, blessName)
				} else {
					optFail("Bless what?")
				}
			case "list":
				if (len(goopt.Args) > 1) {
						listCmd(bucket, strings.TrimSpace(goopt.Args[1]))
					} else {
						listPackagesCmd(bucket)
					}
			case "sync":
				syncCmd()
			default:
				optFail(fmt.Sprintf("Invalid command: %s", cmd))
		}
	} else {
		optFail("Nothing to do")
	}
}
