package main

import "fmt"
import "os"
import "io"
import "time"
import "encoding/binary"
import "encoding/base64"
//import "path"
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

type PackageRepository struct {
	Name string
	BasePath string
}

func (pr *PackageRepository) List() (localRevisions []string) {
	packagePath := filepath.Join(pr.BasePath, pr.Name, "revs")
	
	localRevisions = make([]string, 1000)[0:0]
	
	packageFile, err := os.Open(packagePath)
	if err != nil {
		fmt.Println("Failed to open", packagePath)
		return
	}

	localRevisionFiles, err := packageFile.Readdir(1024)
	if err != nil {
		if err.Error() == "EOF" {
			// Nothing
		} else {
			fmt.Println("Failed to package file", packageFile, err)
			return
		}
	}
	
	for _, fileInfo := range localRevisionFiles {
		localRevisions = append(localRevisions, fileInfo.Name())
	}
	return
}

func (pr *PackageRepository) Add(name string, r io.Reader) (err error)  {
	_ = name
	_ = r
	return
}

func (pr *PackageRepository) Remove(name string) (err error)  {
	_ = name
	return 
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
	revFile := fmt.Sprintf("%s.rev", packageName)
	
	data, err := bucket.Get(revFile)
	if err != nil {
		s3Error, _ := err.(*s3.Error)
		if s3Error.StatusCode == 404 {
			return ""
		} else {
			fmt.Printf("Error finding rev file", err)
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

func downloadPackageRevision(bucket *s3.Bucket, pkg *PackageRepository, revisionName string) {
	fmt.Println("Download", revisionName)
	
	listResp, err := bucket.List(revisionName + ".", ".", "", 1000)
	if err != nil {
		fmt.Println("Failed listing", err)
		return 
	}
	
	//fmt.Println(listResp)
	
	for _, prefix := range listResp.Contents {
		fmt.Println("Found", prefix.Key)
		r, err := bucket.GetReader(prefix.Key)
		if err != nil {
			fmt.Println("Error opening", prefix.Key, err)
			continue
		}
		defer r.Close()
		
		pkg.Add(prefix.Key, r)

		/*
		w, err := os.Create(packagePath + "/" + prefix.Key)
		if err != nil {
			fmt.Println("Failed to create", packagePath + "/" + prefix.Key, err)
			continue
		}
		defer w.Close()
		
		_, err = io.Copy(w, r)
		if err != nil {
			fmt.Println("Failed to copy", err)
			continue
		}
		*/
	}

}

func removePackageRevision(pkg *PackageRepository, revisionName string) {
	fmt.Println("Remove", revisionName)
	_ = pkg.Remove(revisionName)
}

func syncPackage(bucket *s3.Bucket, pkg *PackageRepository) {
	fmt.Println("Syncing", pkg.Name, "to path", pkg.BasePath)
	
	remoteRevisions := listPackageRevisions(bucket, pkg.Name)
	localRevisions := pkg.List()
	
	fmt.Println("Found", len(remoteRevisions), "remote and", len(localRevisions), "local")
	
	remoteNdx, localNdx := 0, 0
	for done := false; !done; {
		if remoteNdx < len(remoteRevisions) {
			fmt.Println("Remote", remoteRevisions[remoteNdx])
		}
		if localNdx < len(localRevisions) {
			fmt.Println("Local", localRevisions[localNdx])
		}
		
		switch {
		case remoteNdx >= len(remoteRevisions) && localNdx >= len(localRevisions):
			done = true
		case remoteNdx >= len(remoteRevisions):
			// We have local revisions, than remote... hmm
			done = true
		case localNdx >= len(localRevisions):
			// We have more remote revisions than local, just download what's left
			downloadPackageRevision(bucket, pkg, remoteRevisions[remoteNdx])
			remoteNdx++
		case remoteRevisions[remoteNdx] > localRevisions[localNdx]:
			// We have an extra local revision, remove it
			removePackageRevision(pkg, localRevisions[localNdx])
			localNdx++
		case remoteRevisions[remoteNdx] < localRevisions[localNdx]:
			// We have a new remote revision, download it
			downloadPackageRevision(bucket, pkg, remoteRevisions[remoteNdx])
			remoteNdx++
		case remoteRevisions[remoteNdx] == localRevisions[localNdx]:
			remoteNdx++
			localNdx++
		}
	}
	
}

func syncCmd(bucket *s3.Bucket, ftlRoot string) {
	rootFile, err := os.Open(ftlRoot)
	if err != nil {
		fmt.Println("Failed to open root", ftlRoot, err)
		return
	}

	dirContents, err := rootFile.Readdir(1024)
	if err != nil {
		if err.Error() == "EOF" {
			// Nothing
		} else {
			fmt.Println("Failed to read root", ftlRoot, err)
			return
		}
	}
	
	for _, file := range dirContents {
		if file.IsDir() {
			pkg := PackageRepository{ftlRoot, file.Name()}
			syncPackage(bucket, &pkg)
		}
	}
	
}

func jumpCmd(bucket *s3.Bucket, revName string) {
	revParts := strings.Split(revName, ".")
	packageName := revParts[0]
	revision := revParts[1]
	
	revFile := fmt.Sprintf("%s.rev", packageName)
	
	err := bucket.Put(revFile, []byte(revision), "text/plain", s3.Private)
	if err != nil {
		fmt.Printf("Failed to put rev file", err)
	}
}

func listPackageRevisions(bucket *s3.Bucket, packageName string) (revisionList []string) {
	revisionList = make([]string, 1000)[0:0]
	listResp, err := bucket.List(packageName + ".", ".", "", 1000)
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

func listCmd(bucket *s3.Bucket, packageName string) {
	activeRev := blessedRevision(bucket, packageName)
	
	for _, revisionName := range listPackageRevisions(bucket, packageName) {
		if len(activeRev) > 0 && strings.HasSuffix(revisionName, activeRev) {
			fmt.Printf("%s\t(active)\n", revisionName)
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
	
	ftlRootEnv := os.Getenv("FTL_ROOT")
	if len(ftlRootEnv) == 0 {
		optFail(fmt.Sprintf("FTL_ROOT not set"))
	}		
	ftlRoot, err := filepath.Abs(ftlRootEnv)
	if err != nil {
		optFail("Invalid FTL_ROOT")
	}
	
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
			case "jump":
				if (len(goopt.Args) > 1) {
					revName := strings.TrimSpace(goopt.Args[1])
					jumpCmd(bucket, revName)
				} else {
					optFail("Jump where?")
				}
			case "list":
				if (len(goopt.Args) > 1) {
						listCmd(bucket, strings.TrimSpace(goopt.Args[1]))
					} else {
						listPackagesCmd(bucket)
					}
			case "sync":
				syncCmd(bucket, ftlRoot)
			default:
				optFail(fmt.Sprintf("Invalid command: %s", cmd))
		}
	} else {
		optFail("Nothing to do")
	}
}
