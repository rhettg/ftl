package ftl

import "fmt"
import "os"
import "strings"
import "syscall"
import "errors"
import "io"
import "path/filepath"

type LocalRepository struct {
	BasePath string
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

func (lr *LocalRepository) Add(name, fileName string, r io.Reader) (err error)  {
	parts := strings.Split(name, ".")
	packageName := parts[0]
	revisionName := parts[1]

	revisionPath := filepath.Join(lr.BasePath, packageName, "revs", revisionName)
	
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
	
	return
}

func (lr *LocalRepository) Remove(name string) (err error)  {
	_ = name
	return 
}

func (lr *LocalRepository) Jump(name string) (err error)  {
	revInfo := NewRevisionInfo(name)
	
	revFileName := filepath.Join(lr.BasePath, revInfo.PackageName, "revs", revInfo.Revision)
	
	activeFileName := lr.activeRevisionFilePath(revInfo.PackageName)
	
	// We have to, maybe, remove the older revision link first.	
	// Note that this isn't atomic, but neither is the ln command
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
	
	return 
}

func (lr *LocalRepository) CheckPackage(packageName string) (err error)  {
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



