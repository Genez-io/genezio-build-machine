package utils

import (
	"fmt"
	"math/rand"
	"os"
	"path"
)

func CreateTempFolder() string {
	tmpDir := os.TempDir()
	folderName := fmt.Sprintf("genezio-%d", os.Getpid())

	err := os.Mkdir(path.Join(tmpDir, folderName), 0755)
	if err != nil {
		fmt.Println(err)
	}

	randomSubfolder := fmt.Sprintf("%d", rand.Int31())
	finalPath := path.Join(tmpDir, folderName, randomSubfolder)
	err = os.Mkdir(finalPath, 0755)
	if err != nil {
		fmt.Println(err)
	}

	return finalPath
}
