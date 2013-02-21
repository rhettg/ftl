package main

import "fmt"
import "os"

//import "path"
import "path/filepath"
import "strings"
import goopt "github.com/droundy/goopt"
import "launchpad.net/goamz/aws"
import "github.com/rhettg/ftl/ftl"

var amVerbose = goopt.Flag([]string{"-v", "--verbose"}, []string{"--quiet"},
	"output verbosely", "be quiet, instead")

var amMaster = goopt.Flag([]string{"--master"}, nil, "Execute against master repository", "")

func optFail(message string) {
	fmt.Println(message)
	fmt.Print(goopt.Help())
	os.Exit(1)
}

func spoolCmd(rr *ftl.RemoteRepository, fileName string) {
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Println("Error opening file", err)
		return
	}

	defer file.Close()

	name := filepath.Base(fileName)
	parts := strings.Split(name, ".")
	packageName := parts[0]

	revisionName, err := rr.Spool(packageName, file)
	if err != nil {
		fmt.Println("Failed to spool", err)
		return
	}

	fmt.Println(revisionName)
}

func downloadPackageRevision(remote *ftl.RemoteRepository, local *ftl.LocalRepository, revisionName string) {
	//packageName := ftl.NewRevisionInfo(revisionName).PackageName

	fileName, r, err := remote.GetRevisionReader(revisionName)
	if err != nil {
		fmt.Println("Failed listing", err)
		return
	}
	if r != nil {
		defer r.Close()
	}

	err = local.Add(revisionName, fileName, r)
	if err != nil {
		fmt.Println("Failed adding", revisionName, err)
		return
	}
}

func removePackageRevision(local *ftl.LocalRepository, revisionName string) {
	fmt.Println("Remove", revisionName)
	_ = local.Remove(revisionName)
}

func syncPackage(remote *ftl.RemoteRepository, local *ftl.LocalRepository, packageName string) {
	err := local.CheckPackage(packageName)
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
			downloadPackageRevision(remote, local, remoteRevisions[remoteNdx])
			remoteNdx++
		case remoteRevisions[remoteNdx] > localRevisions[localNdx]:
			// We have an extra local revision, remove it
			removePackageRevision(local, localRevisions[localNdx])
			localNdx++
		case remoteRevisions[remoteNdx] < localRevisions[localNdx]:
			// We have a new remote revision, download it
			downloadPackageRevision(remote, local, remoteRevisions[remoteNdx])
			remoteNdx++
		case remoteRevisions[remoteNdx] == localRevisions[localNdx]:
			remoteNdx++
			localNdx++
		}
	}

}

func syncCmd(remote *ftl.RemoteRepository, local *ftl.LocalRepository) {
	for _, packageName := range local.ListPackages() {
		syncPackage(remote, local, packageName)
	}

	for _, packageName := range local.ListPackages() {
		activeRev := remote.GetActiveRevision(packageName)
		if len(activeRev) > 0 {
		local.Jump(remote.GetActiveRevision(packageName))
		}
	}
}

func jumpRemoteCmd(remote *ftl.RemoteRepository, revName string) {
	revParts := strings.Split(revName, ".")
	packageName := revParts[0]

	remote.Jump(packageName, revName)
}

func jumpCmd(lr *ftl.LocalRepository, revName string) {
	lr.Jump(revName)
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

func listRemoteCmd(rr *ftl.RemoteRepository, packageName string) {
	activeRev := rr.GetActiveRevision(packageName)

	for _, revisionName := range rr.ListRevisions(packageName) {
		if len(activeRev) > 0 && strings.HasSuffix(revisionName, activeRev) {
			fmt.Printf("%s\t(active)\n", revisionName)
		} else {
			fmt.Println(revisionName)
		}
	}
}

func listPackagesCmd(local *ftl.LocalRepository) {
	for _, revision := range local.ListPackages() {
		fmt.Println(revision)
	}
}

func listRemotePackagesCmd(remote *ftl.RemoteRepository) {
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
	
	ftlBucketEnv := os.Getenv("FTL_BUCKET")
	if len(ftlBucketEnv) == 0 {
		optFail(fmt.Sprintf("FTL_BUCKET not set"))
	}

	remote := ftl.NewRemoteRepository(ftlBucketEnv, auth, aws.USEast)
	local := ftl.NewLocalRepository(ftlRoot)

	if len(goopt.Args) > 0 {
		cmd := strings.TrimSpace(goopt.Args[0])
		switch cmd {
		case "spool":
			if len(goopt.Args) > 1 {
				fileName := strings.TrimSpace(goopt.Args[1])
				fullPath, err := filepath.Abs(fileName)
				if err != nil {
					optFail("Unable to parse path")
				}

				spoolCmd(remote, fullPath)
			} else {
				optFail("Missing file name")
			}
		case "jump":
			if len(goopt.Args) > 1 {
				revName := strings.TrimSpace(goopt.Args[1])

				if *amMaster {
					jumpRemoteCmd(remote, revName)
				} else {
					jumpCmd(local, revName)
				}
			} else {
				optFail("Jump where?")
			}
		case "list":
			if len(goopt.Args) > 1 {
				if *amMaster {
					listRemoteCmd(remote, strings.TrimSpace(goopt.Args[1]))
				} else {
					listCmd(local, strings.TrimSpace(goopt.Args[1]))
				}
			} else {
				if *amMaster {
					listRemotePackagesCmd(remote)
				} else {
					listPackagesCmd(local)
				}
			}
		case "sync":
			syncCmd(remote, local)
		default:
			optFail(fmt.Sprintf("Invalid command: %s", cmd))
		}
	} else {
		optFail("Nothing to do")
	}
}
