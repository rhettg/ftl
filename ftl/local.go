package ftl

import "fmt"
import "os"
import "syscall"
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
	packagePath := filepath.Join(lr.BasePath, "revs")
	
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
		localRevisions = append(localRevisions, fileInfo.Name())
	}
	return
}

func (lr *LocalRepository) activeRevisionFilePath(packageName string) string {
	filePath := filepath.Join(lr.BasePath, packageName, "current")
	return filePath
}

func (lr *LocalRepository) GetActiveRevision(packageName string) (revisionName string) {
	revFilePath := lr.activeRevisionFilePath(packageName)
	
	statInfo, err := os.Lstat(revFilePath)
	if err != nil {
		if pe, ok := err.(*os.PathError); ok {
			err = pe.Err
		}
		
		if err != syscall.ENOENT {
			fmt.Printf("Failed to stat", revFilePath, err)
			return
		}
		
		// Doesn't exist, no revision set
		return
	}
	
	fmt.Println(statInfo)
	
	return
}

func (lr *LocalRepository) Add(name string, r io.Reader) (err error)  {
	_ = name
	_ = r
	return
}

func (lr *LocalRepository) Remove(name string) (err error)  {
	_ = name
	return 
}

func (lr *LocalRepository) Jump(name string) (err error)  {
	_ = name
	return 
}

