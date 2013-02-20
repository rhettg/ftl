package ftl

import "fmt"
import "os"
import "io"
import "path/filepath"

type LocalRepository struct {
	BasePath string
}

func NewLocalRepository(basePath string) (lr *LocalRepository) {
	return &LocalRepository{basePath}
}

func (lr *LocalRepository) ListPackages() (packageNames []string) {
	return
}

func (lr *LocalRepository) ListRevisions(packageName string) (localRevisions []string) {
	packagePath := filepath.Join(lr.BasePath, "revs")
	
	localRevisions = make([]string, 0, 1000)
	
	packageFile, err := os.Open(packagePath)
	if err != nil {
		fmt.Println("Failed to open", packagePath)
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

func (lr *LocalRepository) Add(name string, r io.Reader) (err error)  {
	_ = name
	_ = r
	return
}

func (lr *LocalRepository) Remove(name string) (err error)  {
	_ = name
	return 
}

