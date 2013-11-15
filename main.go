package main

import "fmt"
import "os"

import (
	goopt "github.com/droundy/goopt"
	"github.com/rhettg/ftl/ftl"
	"launchpad.net/goamz/aws"
	"path/filepath"
	"strings"
)

var amVerbose = goopt.Flag([]string{"-v", "--verbose"}, []string{"--quiet"},
	"output verbosely", "be quiet, instead")

var amMaster = goopt.Flag([]string{"--master"}, nil, "Execute against master repository", "")

func optToRegion(regionName string) (region aws.Region) {
	region = aws.USEast

	switch regionName {
	case "us-east":
		region = aws.USEast
	case "us-west-1":
		region = aws.USWest
	case "us-west-2":
		region = aws.USWest2
	}
	return
}

func optFail(message string) {
	fmt.Println(message)
	fmt.Print(goopt.Help())
	os.Exit(1)
}

func spoolCmd(rr *ftl.RemoteRepository, fileName string) error {
	file, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("Error opening file: %v", err)
	}

	defer file.Close()

	name := filepath.Base(fileName)
	parts := strings.Split(name, ".")
	packageName := parts[0]

	revisionName, err := rr.Spool(packageName, file)
	if err != nil {
		return fmt.Errorf("Failed to spool: %v", err)
	}

	fmt.Println(revisionName)
	return nil
}

func downloadPackageRevision(remote *ftl.RemoteRepository, local *ftl.LocalRepository, revisionName string) error {
	//packageName := ftl.NewRevisionInfo(revisionName).PackageName

	fileName, r, err := remote.GetRevisionReader(revisionName)
	if err != nil {
		return fmt.Errorf("Failed listing: %v", err)
	}
	if r != nil {
		defer r.Close()
	}

	err = local.Add(revisionName, fileName, r)
	if err != nil {
		return fmt.Errorf("Failed adding %s: %v", revisionName, err)
	}
	return nil
}

func removePackageRevision(local *ftl.LocalRepository, revisionName string) {
	fmt.Println("Remove", revisionName)
	_ = local.Remove(revisionName)
}

func syncPackage(remote *ftl.RemoteRepository, local *ftl.LocalRepository, packageName string) (err error) {
	err = local.CheckPackage(packageName)
	if err != nil {
		fmt.Println("Package initialize failed", err)
		return
	}

	remoteRevisions := remote.ListRevisions(packageName)
	localRevisions := local.ListRevisions(packageName)

	//fmt.Println("Found", len(remoteRevisions), "remote and", len(localRevisions), "local")

	remoteNdx, localNdx := 0, 0
	for done := false; !done; {
		/*
			if remoteNdx < len(remoteRevisions) {
				fmt.Println("Remote", remoteRevisions[remoteNdx])
			}
			if localNdx < len(localRevisions) {
				fmt.Println("Local", localRevisions[localNdx])
			}
		*/

		switch {
		case remoteNdx >= len(remoteRevisions) && localNdx >= len(localRevisions):
			done = true
		case remoteNdx >= len(remoteRevisions):
			// We have local revisions, than remote... hmm
			done = true
		case localNdx >= len(localRevisions):
			// We have more remote revisions than local, just download what's left
			err = downloadPackageRevision(remote, local, remoteRevisions[remoteNdx])
			remoteNdx++
		case remoteRevisions[remoteNdx] > localRevisions[localNdx]:
			// We have an extra local revision, remove it
			removePackageRevision(local, localRevisions[localNdx])
			localNdx++
		case remoteRevisions[remoteNdx] < localRevisions[localNdx]:
			// We have a new remote revision, download it
			err = downloadPackageRevision(remote, local, remoteRevisions[remoteNdx])
			remoteNdx++
		case remoteRevisions[remoteNdx] == localRevisions[localNdx]:
			remoteNdx++
			localNdx++
		}
	}

	return
}

func syncCmd(remote *ftl.RemoteRepository, local *ftl.LocalRepository) error {
	for _, packageName := range local.ListPackages() {
		err := syncPackage(remote, local, packageName)
		if err != nil {
			return err
		}
	}

	for _, packageName := range local.ListPackages() {
		activeRev, err := remote.GetActiveRevision(packageName)
		if err != nil {
			return err
		}

		if len(activeRev) > 0 {
			activeRev, err := remote.GetActiveRevision(packageName)
			if err != nil {
				return fmt.Errorf("Failed to get Active Revision: %v", err)
			}

			err = local.Jump(activeRev)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func jumpRemoteCmd(remote *ftl.RemoteRepository, revName string) error {
	revParts := strings.Split(revName, ".")
	packageName := revParts[0]

	return remote.Jump(packageName, revName)
}

func jumpCmd(lr *ftl.LocalRepository, revName string) error {
	err := lr.Jump(revName)
	if err != nil {
		return fmt.Errorf("Failed to locally activate %s: %v", revName, err)
	}
	return nil
}

func listCmd(lr *ftl.LocalRepository, packageName string) {
	activeRev := lr.GetActiveRevision(packageName)

	for _, revisionName := range lr.ListRevisions(packageName) {
		if len(activeRev) > 0 && strings.HasSuffix(revisionName, activeRev) {
			fmt.Printf("%s\t(active)\n", revisionName)
		} else {
			fmt.Println(revisionName)
		}
	}
}

func listRemoteCmd(rr *ftl.RemoteRepository, packageName string) error {
	activeRev, err := rr.GetActiveRevision(packageName)
	if err != nil {
		return err
	}

	for _, revisionName := range rr.ListRevisions(packageName) {
		if len(activeRev) > 0 && strings.HasSuffix(revisionName, activeRev) {
			fmt.Printf("%s\t(active)\n", revisionName)
		} else {
			fmt.Println(revisionName)
		}
	}

	return nil
}

func listPackagesCmd(local *ftl.LocalRepository) {
	for _, revision := range local.ListPackages() {
		fmt.Println(revision)
	}
}

func listRemotePackagesCmd(remote *ftl.RemoteRepository) error {
	packageList, err := remote.ListPackages()
	if err != nil {
		return err
	}
	for _, revision := range packageList {
		fmt.Println(revision)
	}
	return nil
}

func main() {
	goopt.Description = func() string {
		return "Faster Than Light Deploy System"
	}
	goopt.Version = "0.3"
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

	ftlBucketEnv := os.Getenv("FTL_BUCKET")
	if len(ftlBucketEnv) == 0 {
		optFail(fmt.Sprintf("FTL_BUCKET not set"))
	}

	remote := ftl.NewRemoteRepository(ftlBucketEnv, auth, optToRegion(os.Getenv("AWS_DEFAULT_REGION")))
	local := ftl.NewLocalRepository(ftlRoot)

	if len(goopt.Args) > 0 {
		cmd := strings.TrimSpace(goopt.Args[0])
		switch cmd {
		case "spool":
			if len(goopt.Args) > 1 {
				fileName := strings.TrimSpace(goopt.Args[1])
				fullPath, e := filepath.Abs(fileName)
				if e != nil {
					optFail("Unable to parse path")
				}

				err = spoolCmd(remote, fullPath)
			} else {
				optFail("Missing file name")
			}
		case "jump":
			if len(goopt.Args) > 1 {
				revName := strings.TrimSpace(goopt.Args[1])

				if *amMaster {
					err = jumpRemoteCmd(remote, revName)
				} else {
					err = jumpCmd(local, revName)
				}
			} else {
				optFail("Jump where?")
			}
		case "list":
			if len(goopt.Args) > 1 {
				if *amMaster {
					err = listRemoteCmd(remote, strings.TrimSpace(goopt.Args[1]))
				} else {
					listCmd(local, strings.TrimSpace(goopt.Args[1]))
				}
			} else {
				if *amMaster {
					err = listRemotePackagesCmd(remote)
				} else {
					listPackagesCmd(local)
				}
			}
		case "sync":
			err = syncCmd(remote, local)
		case "purge":
			if len(goopt.Args) < 2 {
				optFail("Must specify revision to purge")
			}

			revName := strings.TrimSpace(goopt.Args[1])
			if *amMaster {
				err = remote.PurgeRevision(revName)
			} else {
				optFail("I only know how to purge master")
			}

		default:
			optFail(fmt.Sprintf("Invalid command: %s", cmd))
		}
	} else {
		optFail("Nothing to do")
	}

	if err != nil {
		fmt.Println(err)

		if pse, ok := err.(*ftl.PackageScriptError); ok {
			os.Exit(pse.WaitStatus.ExitStatus())
		} else {
			os.Exit(1)
		}
	}
}
