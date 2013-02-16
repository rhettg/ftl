package main

import "fmt"
import "os"
//import "io"
import "path/filepath"
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

func spoolCmd(auth aws.Auth, fileName string) {
	fmt.Println("Working with", fileName)
	
	name := filepath.Base(fileName)
	parts := strings.Split(name, ".")
	nameBase := parts[0]
	ext := parts[1]
	
	revisionId := "1"
	
	myS3 := s3.New(auth, aws.USEast)
	bucket := myS3.Bucket("ftl-rhettg")
	_ = bucket
	
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
	
	s3Path := fmt.Sprintf("%s-%s.%s", nameBase, revisionId, ext)
	bucket.PutReader(s3Path, file, statInfo.Size(), "application/octet-stream", s3.Private)

	/*
	files, err := bucket.List("/", "", "", 10)
	if err != nil {
		fmt.Println("Failed listing", err)
		return
	}
	_ = files
	*/
}

func syncCmd() {
	fmt.Println("Sync")
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
					
					spoolCmd(auth, fullPath)
				} else {
					optFail("Missing file name")
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
