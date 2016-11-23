package ftl

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

const (
	PKG_SCRIPT_POST_SYNC = "post-spool"
	PKG_SCRIPT_PRE_JUMP  = "pre-jump"
	PKG_SCRIPT_POST_JUMP = "post-jump"
	PKG_SCRIPT_UN_JUMP   = "un-jump"
	PKG_SCRIPT_CLEAN     = "clean"
)

type LocalRepository struct {
	BasePath string
}

type PackageScriptError struct {
	WaitStatus syscall.WaitStatus
	Script     string
	Revision   *RevisionInfo
}

func (e *PackageScriptError) Error() string {
	return fmt.Sprintf("Package script %s:%s exited %d", e.Revision, e.Script, e.WaitStatus.ExitStatus())
}

func NewLocalRepository(basePath string) (lr *LocalRepository) {
	return &LocalRepository{basePath}
}

func (lr *LocalRepository) ListPackages() (packageNames []string) {
	repoFile, err := os.Open(lr.BasePath)
	if err != nil {
		fmt.Println("Failed to open", lr.BasePath)
		return
	}

	localPackages, err := repoFile.Readdir(1024)
	if err != nil {
		if err.Error() == "EOF" {
			// Nothing
		} else {
			fmt.Println("Failed to package file", lr.BasePath, err)
			return
		}
	}

	for _, fileInfo := range localPackages {
		packageNames = append(packageNames, fileInfo.Name())
	}
	return
}

func (lr *LocalRepository) ListRevisions(packageName string) (localRevisions []*RevisionInfo) {
	packagePath := filepath.Join(lr.BasePath, packageName, "revs")

	packageFile, err := os.Open(packagePath)
	if err != nil {
		if pe, ok := err.(*os.PathError); ok {
			err = pe.Err
		}

		if err != syscall.ENOENT {
			fmt.Println("Failed to open", packagePath, err)
		}

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

	localRevisionNames := make([]string, 0, 1000)

	for _, fileInfo := range localRevisionFiles {
		localRevisionNames = append(localRevisionNames, fileInfo.Name())
	}

	sort.Strings(localRevisionNames)

	for _, revisionName := range localRevisionNames {
		localRevisions = append(localRevisions, &RevisionInfo{packageName, revisionName})
	}

	return
}

func (lr *LocalRepository) currentRevisionFilePath(packageName string) string {
	filePath := filepath.Join(lr.BasePath, packageName, "current")
	return filePath
}

func (lr *LocalRepository) previousRevisionFilePath(packageName string) string {
	filePath := filepath.Join(lr.BasePath, packageName, "previous")
	return filePath
}

func revisionFromLinkPath(packageName, revisionLinkPath string) (revisionName string) {
	revFilePath, err := os.Readlink(revisionLinkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		fmt.Println("Failed to read current revision", err)
		return
	}

	revisionName = strings.Join([]string{packageName, filepath.Base(revFilePath)}, ".")
	return
}

func (lr *LocalRepository) GetCurrentRevision(packageName string) *RevisionInfo {
	currentFilePath := lr.currentRevisionFilePath(packageName)
	revisionName := revisionFromLinkPath(packageName, currentFilePath)

	if revisionName != "" {
		return NewRevisionInfo(revisionName)
	}

	return nil
}

func (lr *LocalRepository) GetPreviousRevision(packageName string) *RevisionInfo {
	previousFilePath := lr.previousRevisionFilePath(packageName)
	revisionName := revisionFromLinkPath(packageName, previousFilePath)
	if revisionName != "" {
		return &RevisionInfo{packageName, revisionName}
	}

	return nil
}

func (lr *LocalRepository) Add(revision *RevisionInfo, fileName string, r io.Reader) (err error) {
	revisionPath := filepath.Join(lr.BasePath, revision.PackageName, "revs", revision.Revision)
	fmt.Println("Adding", revisionPath)

	err = os.MkdirAll(revisionPath, 0755)
	if err != nil {
		return
	}

	revisionFilePath := filepath.Join(revisionPath, fileName)
	w, err := os.Create(revisionFilePath)
	if err != nil {
		return
	}

	defer w.Close()

	_, err = io.Copy(w, r)
	if err != nil {
		return
	}

	w.Close()

	checkFile, err := os.Open(revisionFilePath)
	hashPrefix, err := fileHashPrefix(checkFile)
	if err != nil {
		return
	}

	if hashPrefix != revision.Name()[len(revision.Name())-2:] {
		return fmt.Errorf("Checksum does not match")
	}

	if strings.HasSuffix(fileName, ".tgz") || strings.HasSuffix(fileName, ".gz") {
		cmd := exec.Command("gunzip", revisionFilePath)
		err = cmd.Run()
		if err != nil {
			fmt.Println("Failed to unzip", err)
			return
		}
	}

	revisionFilePrefix := filepath.Join(revisionPath, revision.Name())

	_, err = os.Stat(revisionFilePrefix + ".tar")
	if err == nil {
		cmd := exec.Command("tar", "-C", revisionPath, "-xf", revisionFilePrefix+".tar")
		err = cmd.Run()
		if err != nil {
			fmt.Println("Failed to untar", err)
			return
		}
		err = os.Remove(revisionFilePrefix + ".tar")
		if err != nil {
			fmt.Println("Failed to cleanup", err)
			return
		}
	}

	err = lr.RunPackageScript(revision, PKG_SCRIPT_POST_SYNC)
	if err != nil {
		return
	}

	return
}

func (lr *LocalRepository) Remove(revision *RevisionInfo) error {
	activeRevision := lr.GetCurrentRevision(revision.PackageName)
	if activeRevision != nil && *activeRevision == *revision {
		return fmt.Errorf("Can't remove active revision")
	}

	revFileName := filepath.Join(lr.BasePath, revision.PackageName, "revs", revision.Revision)
	e := os.RemoveAll(revFileName)
	if e != nil {
		return fmt.Errorf("Failed to remove %v: %v", revFileName, e)
	}

	return nil
}

func (lr *LocalRepository) SetPreviousJump(revision *RevisionInfo) (err error) {
	existingRevision := lr.GetPreviousRevision(revision.PackageName)
	if existingRevision != nil && *existingRevision == *revision {
		// Already set
		return
	}

	newRevisionPath := filepath.Join(lr.BasePath, revision.PackageName, "revs", revision.Revision)
	_, err = os.Stat(newRevisionPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Revision doesn't exist")
			return
		}
	}

	previousLinkPath := lr.previousRevisionFilePath(revision.PackageName)

	err = os.Remove(previousLinkPath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Println("Failed to remove old previous")
			return
		}
	}

	err = os.Symlink(newRevisionPath, previousLinkPath)
	if err != nil {
		fmt.Println("Failed creating symlink", err)
		return
	}

	return
}

func (lr *LocalRepository) Jump(revision *RevisionInfo) (err error) {
	existingRevision := lr.GetCurrentRevision(revision.PackageName)
	if existingRevision != nil && *existingRevision == *revision {
		// Already active
		return
	}

	newRevisionPath := filepath.Join(lr.BasePath, revision.PackageName, "revs", revision.Revision)
	_, err = os.Stat(newRevisionPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Revision doesn't exist")
			return
		}
	}

	err = lr.RunPackageScript(revision, PKG_SCRIPT_PRE_JUMP)
	if err != nil {
		return
	}

	currentLinkPath := lr.currentRevisionFilePath(revision.PackageName)
	if existingRevision != nil {
		err = lr.RunPackageScript(existingRevision, PKG_SCRIPT_UN_JUMP)
		if err != nil {
			return
		}

		err = lr.SetPreviousJump(existingRevision)
		if err != nil {
			return
		}

		// We have to, maybe, remove the older revision link first.
		// Note that this isn't atomic, but neither is the ln command
		err = os.Remove(currentLinkPath)
		if err != nil {
			if !os.IsNotExist(err) {
				fmt.Println("Failed to remove old version")
				return
			}
		}
	}

	err = os.Symlink(newRevisionPath, currentLinkPath)
	if err != nil {
		fmt.Println("Failed creating symlink", err)
		return
	}

	err = lr.RunPackageScript(revision, PKG_SCRIPT_POST_JUMP)
	if err != nil {
		return
	}

	return
}

func (lr *LocalRepository) JumpBack(pkgName string) error {
	currentLinkPath := lr.currentRevisionFilePath(pkgName)
	previousLinkPath := lr.previousRevisionFilePath(pkgName)

	currentRevision := lr.GetCurrentRevision(pkgName)
	previousRevision := lr.GetPreviousRevision(pkgName)

	previousRevPath, err := os.Readlink(previousLinkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("There is no previous revision")
		}
		return fmt.Errorf("Failed to read previous version: %v", err)
	}

	currentRevPath, err := os.Readlink(currentLinkPath)
	if err != nil {
		return fmt.Errorf("Failed to read current version: %v", err)
	}

	err = lr.RunPackageScript(previousRevision, PKG_SCRIPT_PRE_JUMP)
	if err != nil {
		return err
	}

	err = lr.RunPackageScript(currentRevision, PKG_SCRIPT_UN_JUMP)
	if err != nil {
		return err
	}

	err = os.Remove(currentLinkPath)
	if err != nil {
		return fmt.Errorf("Failed to remove current version: %v", err)
	}

	err = os.Symlink(previousRevPath, currentLinkPath)
	if err != nil {
		return fmt.Errorf("Failed creating symlink", err)
	}

	err = os.Remove(previousLinkPath)
	if err != nil {
		return fmt.Errorf("Failed to remove previous version: %v", err)
	}

	err = os.Symlink(currentRevPath, previousLinkPath)
	if err != nil {
		fmt.Println("Failed creating symlink", err)
	}

	err = lr.RunPackageScript(previousRevision, PKG_SCRIPT_POST_JUMP)
	if err != nil {
		return err
	}

	return nil
}

func (lr *LocalRepository) CheckPackage(packageName string) (err error) {
	pkgPath := filepath.Join(lr.BasePath, packageName)

	// Make sure we have the right files
	statInfo, err := os.Stat(pkgPath)
	if err != nil {
		fmt.Printf("Package doesn't exist", pkgPath, err)
		return
	}

	if !statInfo.IsDir() {
		err = errors.New("Not a directory: " + pkgPath)
		return
	}

	revPath := filepath.Join(lr.BasePath, packageName, "revs")
	revInfo, err := os.Stat(revPath)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(revPath, 0755)
			if err != nil {
				fmt.Printf("Failed to create rev path", revPath, err)
				err = errors.New("Failed to create revs path")
				return
			}
		} else {
			fmt.Printf("Error opening revs directory")
			return
		}
	} else {
		if !revInfo.IsDir() {
			fmt.Printf("Not a directory", revPath)
			err = errors.New("Bad revs path")
			return
		}
	}

	return
}

func (lr *LocalRepository) RunPackageScript(revision *RevisionInfo, scriptName string) (err error) {
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)

	revPath := filepath.Join(lr.BasePath, revision.PackageName, "revs", revision.Revision)
	scriptPath := filepath.Join(revPath, "ftl", scriptName)

	os.Chdir(revPath)

	_, err = os.Stat(scriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			// That's fine, script isn't required.
			err = nil
		}
		return
	}

	cmd := exec.Command(scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if e, ok := err.(*exec.ExitError); ok {
		if s, ok := e.Sys().(syscall.WaitStatus); ok {
			err = &PackageScriptError{s, scriptName, revision}
		}
	}
	return
}
