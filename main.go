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

const Version = "0.1.0"

var amVerbose = goopt.Flag([]string{"-v", "--verbose"}, []string{"--quiet"},
	"output verbosely", "be quiet, instead")

var amMaster = goopt.Flag([]string{"--master"}, nil, "Execute against master repository", "")

var amVersion = goopt.Flag([]string{"--version"}, nil, "Display current version", "")

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

	revision, err := rr.Spool(packageName, file)
	if err != nil {
		return fmt.Errorf("Failed to spool: %v", err)
	}

	fmt.Println(revision.Name())
	return nil
}

func downloadPackageRevision(remote *ftl.RemoteRepository, local *ftl.LocalRepository, revision ftl.RevisionInfo) error {
	fileName, r, err := remote.GetRevisionReader(revision)
	if err != nil {
		return fmt.Errorf("Failed listing: %v", err)
	}
	if r != nil {
		defer r.Close()
	}

	err = local.Add(revision, fileName, r)
	if err != nil {
		return fmt.Errorf("Failed adding %s: %v", revision.Name(), err)
	}
	return nil
}

func removePackageRevision(local *ftl.LocalRepository, revision ftl.RevisionInfo) error {
	fmt.Println("Remove", revision.Name())
	return local.Remove(revision)
}

func syncPackage(remoteRevisions, localRevisions []ftl.RevisionInfo, startRev ftl.RevisionInfo) (downloadRevs, purgeRevs []ftl.RevisionInfo, err error) {
	downloadRevs = []ftl.RevisionInfo{}
	purgeRevs = []ftl.RevisionInfo{}

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
			// We have more local revisions, than remote... hmm
			done = true
		case localNdx >= len(localRevisions):
			// We have more remote revisions than local, just download what's left
			downloadRevs = append(downloadRevs, remoteRevisions[remoteNdx])
			remoteNdx++
		case remoteRevisions[remoteNdx] == localRevisions[localNdx]:
			remoteNdx++
			localNdx++
		case remoteRevisions[remoteNdx].Revision < startRev.Revision:
			// To early for us, carry on
			remoteNdx++
		case remoteRevisions[remoteNdx].Revision < localRevisions[localNdx].Revision:
			// We have a new remote revision, download it
			downloadRevs = append(downloadRevs, remoteRevisions[remoteNdx])
			remoteNdx++
		case remoteRevisions[remoteNdx].Revision > localRevisions[localNdx].Revision:
			// We have an extra local revision, remove it
			purgeRevs = append(purgeRevs, localRevisions[localNdx])
			localNdx++
		}
	}

	return
}

func syncCmd(remote *ftl.RemoteRepository, local *ftl.LocalRepository) error {
	for _, packageName := range local.ListPackages() {

		err := local.CheckPackage(packageName)
		if err != nil {
			fmt.Println("Package initialize failed", err)
			return err
		}

		crChan := make(chan ftl.RevisionListResult)
		go func() {
			currentRev, err := remote.GetCurrentRevision(packageName)
			crChan <- ftl.RevisionListResult{[]ftl.RevisionInfo{currentRev}, err}
		}()

		crResult := <-crChan
		if crResult.Err != nil {
			return crResult.Err
		}

		currentRev := crResult.Revisions[0]

		previousRev, err := remote.GetPreviousRevision(packageName)
		if err != nil {
			return err
		}

		firstRev := previousRev
		if currentRev.Revision < firstRev.Revision {
			firstRev = currentRev
		}

		localRevisions := local.ListRevisions(packageName)

		remoteRevisions, err := remote.ListRevisions(packageName)
		if err != nil {
			return err
		}

		download, purge, err := syncPackage(remoteRevisions, localRevisions, firstRev)
		if err != nil {
			return err
		}

		for _, rev := range download {
			err = downloadPackageRevision(remote, local, rev)
			if err != nil {
				return err
			}
		}

		if len(currentRev.Revision) > 0 {
			currentRev, err := remote.GetCurrentRevision(packageName)
			if err != nil {
				return fmt.Errorf("Failed to get Active Revision: %v", err)
			}

			err = local.Jump(currentRev)
			if err != nil {
				return err
			}
		}

		for _, rev := range purge {
			err = removePackageRevision(local, rev)
			if err != nil {
				return err
			}
		}

	}
	return nil
}

func jumpCmd(lr *ftl.LocalRepository, revision ftl.RevisionInfo) error {
	err := lr.Jump(revision)
	if err != nil {
		return fmt.Errorf("Failed to locally activate %s: %v", revision.Name(), err)
	}
	return nil
}

func listCmd(lr *ftl.LocalRepository, packageName string) {
	activeRev := lr.GetCurrentRevision(packageName)

	for _, revision := range lr.ListRevisions(packageName) {
		if len(activeRev.Revision) > 0 && revision == activeRev {
			fmt.Printf("%s\t(active)\n", revision.Name())
		} else {
			fmt.Println(revision.Name())
		}
	}
}

func listRemoteCmd(rr *ftl.RemoteRepository, packageName string) error {
	activeRev, err := rr.GetCurrentRevision(packageName)
	if err != nil {
		return err
	}

	revisionList, err := rr.ListRevisions(packageName)
	if err != nil {
		return err
	}

	for _, revision := range revisionList {
		if len(activeRev.Revision) > 0 && activeRev == revision {
			fmt.Printf("%s\t(active)\n", revision.Name())
		} else {
			fmt.Println(revision.Name())
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
	goopt.Version = Version
	goopt.Summary = "Deploy system built around S3."
	goopt.Parse(nil)

	if *amVersion {
		fmt.Println(Version)
		os.Exit(0)
	}

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
				revision := *ftl.NewRevisionInfo(strings.TrimSpace(goopt.Args[1]))

				if *amMaster {
					err = remote.Jump(revision)
				} else {
					err = local.Jump(revision)
				}
			} else {
				optFail("Jump where?")
			}
		case "jump-back":
			if len(goopt.Args) > 1 {
				pkgName := strings.TrimSpace(goopt.Args[1])
				if *amMaster {
					err = remote.JumpBack(pkgName)
				} else {
					err = local.JumpBack(pkgName)
				}
			} else {
				optFail("Package name required")
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

			revision := *ftl.NewRevisionInfo(strings.TrimSpace(goopt.Args[1]))
			if *amMaster {
				err = remote.PurgeRevision(revision)
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
