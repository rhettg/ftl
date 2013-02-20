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
import "github.com/rhettg/ftl/ftl"

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

func downloadPackageRevision(remote *ftl.RemoteRepository, pkg *ftl.PackageRepository, revisionName string) {
	r, err := remote.GetRevisionReader(pkg.Name, revisionName)
	if err != nil {
		fmt.Println("Failed listing", err)
		return 
	}
	defer r.Close()
	
	err = pkg.Add(revisionName, r)
	if err != nil {
		fmt.Println("Failed listing", err)
		return 
	}
}

func removePackageRevision(pkg *ftl.PackageRepository, revisionName string) {
	fmt.Println("Remove", revisionName)
	_ = pkg.Remove(revisionName)
}

func syncPackage(remote *ftl.RemoteRepository, pkg *ftl.PackageRepository) {
	fmt.Println("Syncing", pkg.Name, "to path", pkg.BasePath)
	
	remoteRevisions := remote.ListRevisions(pkg.Name)
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
			downloadPackageRevision(remote, pkg, remoteRevisions[remoteNdx])
			remoteNdx++
		case remoteRevisions[remoteNdx] > localRevisions[localNdx]:
			// We have an extra local revision, remove it
			removePackageRevision(pkg, localRevisions[localNdx])
			localNdx++
		case remoteRevisions[remoteNdx] < localRevisions[localNdx]:
			// We have a new remote revision, download it
			downloadPackageRevision(remote, pkg, remoteRevisions[remoteNdx])
			remoteNdx++
		case remoteRevisions[remoteNdx] == localRevisions[localNdx]:
			remoteNdx++
			localNdx++
		}
	}
	
}

func syncCmd(remote *ftl.RemoteRepository, ftlRoot string) {
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
			pkg := ftl.PackageRepository{ftlRoot, file.Name()}
			syncPackage(remote, &pkg)
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

func listCmd(rr *ftl.RemoteRepository, packageName string) {
	activeRev := rr.GetBlessedRevision(packageName)
	
	for _, revisionName := range rr.ListRevisions(packageName) {
		if len(activeRev) > 0 && strings.HasSuffix(revisionName, activeRev) {
			fmt.Printf("%s\t(active)\n", revisionName)
		} else {
			fmt.Println(revisionName)
		}
	}
}

func listPackagesCmd(remote *ftl.RemoteRepository) {
	for _, revision := range remote.ListPackages() {
		fmt.Println(revision)
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
	
	remote := ftl.NewRemoteRepository("ftl-rhettg", auth, aws.USEast)
	_ = remote
	
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
						listCmd(remote, strings.TrimSpace(goopt.Args[1]))
					} else {
						listPackagesCmd(remote)
					}
			case "sync":
				syncCmd(remote, ftlRoot)
			default:
				optFail(fmt.Sprintf("Invalid command: %s", cmd))
		}
	} else {
		optFail("Nothing to do")
	}
}
