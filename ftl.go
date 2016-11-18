package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/urfave/cli"

	"github.com/rhettg/ftl/ftl"
)

const DOWNLOAD_WORKERS = 4

const Version = "0.3.0"

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

func downloadPackageRevision(remote *ftl.RemoteRepository, local *ftl.LocalRepository, revision *ftl.RevisionInfo) error {
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

func removePackageRevision(local *ftl.LocalRepository, revision *ftl.RevisionInfo) error {
	fmt.Println("Remove", revision.Name())
	return local.Remove(revision)
}

func syncPackage(remoteRevisions, localRevisions []*ftl.RevisionInfo, startRev *ftl.RevisionInfo) (downloadRevs, purgeRevs []*ftl.RevisionInfo, err error) {
	remoteNdx, localNdx := 0, 0
	for done := false; !done; {
		switch {
		case remoteNdx >= len(remoteRevisions) && localNdx >= len(localRevisions):
			done = true
		case remoteNdx >= len(remoteRevisions):
			// We have more local revisions, than remote... hmm
			purgeRevs = append(purgeRevs, localRevisions[localNdx])
			localNdx++
		case localNdx < len(localRevisions) && remoteRevisions[remoteNdx].Revision == localRevisions[localNdx].Revision:
			remoteNdx++
			localNdx++
		case startRev != nil && remoteRevisions[remoteNdx].Revision < startRev.Revision:
			// To early for us, carry on
			remoteNdx++
		case localNdx >= len(localRevisions):
			// We have more remote revisions than local, just download what's left
			downloadRevs = append(downloadRevs, remoteRevisions[remoteNdx])
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

func retrieveRemoteRevisions(r *ftl.RemoteRepository, packageName string) (curRev, prevRev *ftl.RevisionInfo, revisions []*ftl.RevisionInfo, err error) {
	crChan := make(chan ftl.RevisionListResult)
	go func() {
		currentRev, err := r.GetCurrentRevision(packageName)
		crChan <- ftl.RevisionListResult{[]*ftl.RevisionInfo{currentRev}, err}
	}()

	prChan := make(chan ftl.RevisionListResult)
	go func() {
		previousRev, err := r.GetPreviousRevision(packageName)
		prChan <- ftl.RevisionListResult{[]*ftl.RevisionInfo{previousRev}, err}
	}()

	rrChan := make(chan ftl.RevisionListResult)
	go func() {
		remoteRevisions, err := r.ListRevisions(packageName)
		rrChan <- ftl.RevisionListResult{remoteRevisions, err}
	}()

	crResult := <-crChan
	if crResult.Err != nil {
		err = fmt.Errorf("Failed to retrieve current revision")
	} else {
		curRev = crResult.Revisions[0]
	}

	prResult := <-prChan
	if prResult.Err != nil {
		err = fmt.Errorf("Failed to retrieve previous revision")
	} else {
		prevRev = prResult.Revisions[0]
	}

	rrResult := <-rrChan
	if rrResult.Err != nil {
		err = fmt.Errorf("Failed to retrieve remote revisions")
	} else {
		revisions = rrResult.Revisions
	}

	return
}

func downloadRemoteRevisions(r *ftl.RemoteRepository, l *ftl.LocalRepository, revisions []*ftl.RevisionInfo) error {
	workerChan := make(chan bool, DOWNLOAD_WORKERS)
	for i := 0; i < DOWNLOAD_WORKERS; i++ {
		workerChan <- true
	}

	downloadChan := make(chan error)
	for _, rev := range revisions {
		rev := rev
		go func() {
			<-workerChan
			downloadChan <- downloadPackageRevision(r, l, rev)
			workerChan <- true
		}()
	}

	errList := make([]error, 0, len(revisions))
	for _ = range revisions {
		err := <-downloadChan
		errList = append(errList, err)
	}

	for _, err := range errList {
		if err != nil {
			return fmt.Errorf("Failed downloading revisions")
		}
	}

	return nil
}

func syncCmd(remote *ftl.RemoteRepository, local *ftl.LocalRepository) error {
	for _, packageName := range local.ListPackages() {
		err := local.CheckPackage(packageName)
		if err != nil {
			fmt.Println("Package initialize failed", err)
			return err
		}

		curRev, prevRev, remoteRevisions, err := retrieveRemoteRevisions(remote, packageName)
		if err != nil {
			return err
		}

		var firstRev *ftl.RevisionInfo = nil
		if prevRev != nil {
			firstRev = prevRev
			// Special case for post-jump-back, where prev might be more
			// current than cur
			if curRev != nil && curRev.Revision < firstRev.Revision {
				firstRev = curRev
			}
		}

		localRevisions := local.ListRevisions(packageName)

		download, purge, err := syncPackage(remoteRevisions, localRevisions, firstRev)
		if err != nil {
			return err
		}

		err = downloadRemoteRevisions(remote, local, download)

		if curRev != nil {
			err = local.Jump(curRev)
			if err != nil {
				return err
			}
		}

		if prevRev != nil {
			err = local.SetPreviousJump(prevRev)
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

func jumpCmd(lr *ftl.LocalRepository, revision *ftl.RevisionInfo) error {
	err := lr.Jump(revision)
	if err != nil {
		return fmt.Errorf("Failed to locally activate %s: %v", revision.Name(), err)
	}
	return nil
}

func listCmd(lr *ftl.LocalRepository, packageName string) {
	activeRev := lr.GetCurrentRevision(packageName)

	for _, revision := range lr.ListRevisions(packageName) {
		if activeRev != nil && *revision == *activeRev {
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
		if activeRev != nil && *activeRev == *revision {
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

// Translate PackageScriptErrors into command line errors so we get the
// correct exit status
func capturePackageScriptError(err error) error {
	if pse, ok := err.(*ftl.PackageScriptError); ok {
		return cli.NewExitError(pse.Error(), pse.WaitStatus.ExitStatus())
	} else {
		return err
	}
}

func newRemoteRepository(c *cli.Context) (remote *ftl.RemoteRepository, err error) {
	if len(c.String("aws-region")) == 0 {
		err = errors.New("AWS_DEFAULT_REGION not set")
		return
	}

	if len(c.String("ftl-bucket")) == 0 {
		err = errors.New("FTL_BUCKET not set")
		return
	}

	s := session.New(&aws.Config{Region: aws.String(c.String("aws-region"))})
	remote = ftl.NewRemoteRepository(c.String("ftl-bucket"), s)
	return
}

func newLocalRepository(c *cli.Context) (local *ftl.LocalRepository, err error) {
	fmt.Println(c.String("ftl-root"))
	if len(c.String("ftl-root")) == 0 {
		err = errors.New("FTL_ROOT not set")
		return
	}

	ftlRoot, err := filepath.Abs(c.String("ftl-root"))
	if err != nil {
		return
	}

	local = ftl.NewLocalRepository(ftlRoot)
	return
}

func main() {
	app := cli.NewApp()
	app.Name = "ftl"
	app.Usage = "Faster Than Light Deploy System"
	app.Version = Version

	rootFlag := cli.StringFlag{
		Name:   "ftl-root",
		Usage:  "Path to local repository",
		EnvVar: "FTL_ROOT",
	}
	bucketFlag := cli.StringFlag{
		Name:   "ftl-bucket",
		Usage:  "S3 Bucket for remote repository",
		EnvVar: "FTL_BUCKET",
	}
	regionFlag := cli.StringFlag{
		Name:   "aws-region",
		Usage:  "AWS Region for S3 Bucket",
		EnvVar: "AWS_DEFAULT_REGION",
	}
	executeRemoteFlag := cli.BoolFlag{
		Name:  "master, remote",
		Usage: "Execute against remote revision repository",
	}

	var err error
	var remote *ftl.RemoteRepository
	var local *ftl.LocalRepository

	app.Commands = []cli.Command{
		{
			Name:    "spool",
			Aliases: []string{"s"},
			Usage:   "",
			Flags: []cli.Flag{
				bucketFlag,
				regionFlag,
				executeRemoteFlag,
			},
			Action: func(c *cli.Context) error {
				if c.NArg() > 0 {
					fileName := c.Args().First()
					fullPath, e := filepath.Abs(fileName)
					if e != nil {
						return cli.NewExitError("Unable to parse path", 1)
					}

					remote, err := newRemoteRepository(c)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					return spoolCmd(remote, fullPath)
				} else {
					return cli.NewExitError("Missing file name", 1)
				}
			},
		},
		{
			Name:    "jump",
			Aliases: []string{"j"},
			Usage:   "Activate the specified revision",
			Flags: []cli.Flag{
				rootFlag,
				bucketFlag,
				regionFlag,
				executeRemoteFlag,
			},
			Action: func(c *cli.Context) error {
				if c.NArg() > 0 {
					revision := ftl.NewRevisionInfo(c.Args().First())

					if revision == nil {
						return cli.NewExitError("Invalid revision name", 1)
					} else if c.Bool("remote") {

						remote, err = newRemoteRepository(c)
						if err != nil {
							return cli.NewExitError(err.Error(), 1)
						}
						err = remote.Jump(revision)
					} else {
						local, err = newLocalRepository(c)
						if err != nil {
							return cli.NewExitError(err.Error(), 1)
						}
						err = local.Jump(revision)
					}
				} else {
					return cli.NewExitError("Jump where?", 1)
				}
				return capturePackageScriptError(err)
			},
		},
		{
			Name:    "jump-back",
			Aliases: []string{"b"},
			Usage:   "Activate the previous revision",
			Flags: []cli.Flag{
				rootFlag,
				bucketFlag,
				regionFlag,
				executeRemoteFlag,
			},
			Action: func(c *cli.Context) error {
				if c.NArg() > 0 {
					pkgName := c.Args().First()
					if c.Bool("remote") {
						remote, err = newRemoteRepository(c)
						if err != nil {
							return cli.NewExitError(err.Error(), 1)
						}

						err = remote.JumpBack(pkgName)
					} else {
						local, err = newLocalRepository(c)
						if err != nil {
							return cli.NewExitError(err.Error(), 1)
						}

						err = local.JumpBack(pkgName)
					}
				} else {
					return cli.NewExitError("Package name required", 1)
				}
				return capturePackageScriptError(err)
			},
		},
		{
			Name:    "list",
			Aliases: []string{"l"},
			Usage:   "List available revisions",
			Flags: []cli.Flag{
				rootFlag,
				bucketFlag,
				regionFlag,
				executeRemoteFlag,
			},
			Action: func(c *cli.Context) error {
				if c.Bool("remote") {
					remote, err = newRemoteRepository(c)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
				} else {
					local, err = newLocalRepository(c)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
				}

				if c.NArg() > 0 {
					if c.Bool("remote") {
						return listRemoteCmd(remote, c.Args().First())
					} else {
						listCmd(local, c.Args().First())
					}
				} else {
					if c.Bool("remote") {
						return listRemotePackagesCmd(remote)
					} else {
						listPackagesCmd(local)
					}
				}
				return nil
			},
		},
		{
			Name:    "sync",
			Aliases: []string{"s"},
			Usage:   "",
			Flags: []cli.Flag{
				rootFlag,
				bucketFlag,
				regionFlag,
			},
			Action: func(c *cli.Context) error {
				remote, err = newRemoteRepository(c)
				if err != nil {
					return cli.NewExitError(err.Error(), 1)
				}

				local, err = newLocalRepository(c)
				if err != nil {
					return cli.NewExitError(err.Error(), 1)
				}

				err = syncCmd(remote, local)
				return capturePackageScriptError(err)
			},
		},
		{
			Name:    "purge",
			Aliases: []string{"p"},
			Usage:   "",
			Flags: []cli.Flag{
				bucketFlag,
				regionFlag,
				executeRemoteFlag,
			},
			Action: func(c *cli.Context) error {
				if c.NArg() == 0 {
					return cli.NewExitError("Must specify revision to purge", 1)
				}

				revision := ftl.NewRevisionInfo(c.Args().First())
				if revision == nil {
					return cli.NewExitError("Package name required", 1)
				} else if c.Bool("remote") {
					remote, err = newRemoteRepository(c)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					return remote.PurgeRevision(revision)
				} else {
					return cli.NewExitError("I only know how to purge master", 1)
				}
			},
		},
	}

	app.Run(os.Args)
}
