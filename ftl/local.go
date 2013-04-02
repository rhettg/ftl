package ftl

import "fmt"
import "os"
import "sort"
import "os/exec"
import "strings"
import "syscall"
import "errors"
import "io"
import "path/filepath"

const (
	PKG_SCRIPT_POST_SYNC = "post-spool"
	PKG_SCRIPT_PRE_JUMP = "pre-jump"
	PKG_SCRIPT_POST_JUMP = "post-jump"
	PKG_SCRIPT_UN_JUMP = "un-jump"
	PKG_SCRIPT_CLEAN = "clean"
)

type LocalRepository struct {
	BasePath string
}

type PackageScriptError struct {
	WaitStatus syscall.WaitStatus
	Script string
	Revision string
}

func (e *PackageScriptError) Error() string {
	return fmt.Sprintf("Package script %s:%s exited %d", e.Revision, e.Script, e.WaitStatus.ExitStatus())
}


func NewLocalRepository(basePath string) (lr *LocalRepository) {
	return &LocalRepository{basePath}
}

func (lr *LocalRepository) ListPackages() (packageNames []string) {
	packageNames = make([]string, 0, 1000)

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

func (lr *LocalRepository) ListRevisions(packageName string) (localRevisions []string) {
	packagePath := filepath.Join(lr.BasePath, packageName, "revs")

	localRevisions = make([]string, 0, 1000)

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

	for _, fileInfo := range localRevisionFiles {
		localRevisions = append(localRevisions, strings.Join([]string{packageName, fileInfo.Name()}, "."))
	}
	
	sort.Strings(localRevisions)
	return
}

func (lr *LocalRepository) activeRevisionFilePath(packageName string) string {
	filePath := filepath.Join(lr.BasePath, packageName, "current")
	return filePath
}

func (lr *LocalRepository) GetActiveRevision(packageName string) (revisionName string) {
	activeFilePath := lr.activeRevisionFilePath(packageName)

	revFilePath, err := os.Readlink(activeFilePath)
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

func (lr *LocalRepository) Add(name, fileName string, r io.Reader) (err error) {
	parts := strings.Split(name, ".")
	packageName := parts[0]
	revisionName := parts[1]

	revisionPath := filepath.Join(lr.BasePath, packageName, "revs", revisionName)
	fmt.Println("Adding", revisionPath)

	err = os.Mkdir(revisionPath, 0755)
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

	// TODO: Check MD5 suffix

	if strings.HasSuffix(fileName, ".tgz") || strings.HasSuffix(fileName, ".gz") {
		cmd := exec.Command("gunzip", revisionFilePath)
		err = cmd.Run()
		if err != nil {
			fmt.Println("Failed to unzip", err)
			return
		}
	}

	revisionFilePrefix := filepath.Join(revisionPath, name)

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
	
	err = lr.RunPackageScript(name, PKG_SCRIPT_POST_SYNC)
	if err != nil {
		return
	}

	return
}

func (lr *LocalRepository) Remove(name string) (err error) {
	_ = name
	return
}

func (lr *LocalRepository) Jump(name string) (err error) {
	revInfo := NewRevisionInfo(name)

	if lr.GetActiveRevision(revInfo.PackageName) == name {
		// Already active
		return
	}

	revFileName := filepath.Join(lr.BasePath, revInfo.PackageName, "revs", revInfo.Revision)
	_, err = os.Stat(revFileName)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Revision doesn't exist")
			return
		}
	}
	
	err = lr.RunPackageScript(name, PKG_SCRIPT_PRE_JUMP)
	if err != nil {
		return
	}
	
	
	activeRevision := lr.GetActiveRevision(revInfo.PackageName)
	if len(activeRevision) > 0 {
	err = lr.RunPackageScript(activeRevision, PKG_SCRIPT_UN_JUMP)
	if err != nil {
		return
	}
	}

	// We have to, maybe, remove the older revision link first.	
	// Note that this isn't atomic, but neither is the ln command
	activeFileName := lr.activeRevisionFilePath(revInfo.PackageName)
	err = os.Remove(activeFileName)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Println("Failed to remove old version")
			return
		}

	}
	err = os.Symlink(revFileName, activeFileName)
	if err != nil {
		fmt.Println("Failed creating symlink", err)
	}

	err = lr.RunPackageScript(name, PKG_SCRIPT_POST_JUMP)
	if err != nil {
		return
	}

	return
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

func (lr *LocalRepository) RunPackageScript(revisionName, scriptName string) (err error) {
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)


	revInfo := NewRevisionInfo(revisionName)

	revPath := filepath.Join(lr.BasePath, revInfo.PackageName, "revs", revInfo.Revision)
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
			err = &PackageScriptError{s, scriptName, revisionName}
		}
	}
	return
}
