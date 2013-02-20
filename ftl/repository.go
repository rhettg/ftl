package ftl

import "fmt"
import "os"
import "io"
import "path/filepath"

type PackageRepository struct {
	Name string
	BasePath string
}

func (pr *PackageRepository) List() (localRevisions []string) {
	packagePath := filepath.Join(pr.BasePath, pr.Name, "revs")
	
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

func (pr *PackageRepository) Add(name string, r io.Reader) (err error)  {
	_ = name
	_ = r
	return
}

func (pr *PackageRepository) Remove(name string) (err error)  {
	_ = name
	return 
}

