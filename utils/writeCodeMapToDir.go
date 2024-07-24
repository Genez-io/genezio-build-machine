package utils

import (
	"log"
	"os"
	"path"
	"strings"
)

func WriteCodeMapToDirAndZip(code map[string]string, tmpFolderPath string) (string, error) {
	// Write code to temp folder
	for fileName, fileContent := range code {
		filePath := path.Join(tmpFolderPath, fileName)
		log.Default().Println("Writing file", fileName, "to", tmpFolderPath)

		// Check if file is in a subfolder
		if strings.Contains(fileName, "/") {
            log.Println("Creating subfolder", path.Dir(filePath))
			err := os.MkdirAll(path.Dir(filePath), 0755)
			if err != nil {
				return "", err
			}
		}

		err := os.WriteFile(filePath, []byte(fileContent), 0644)
		if err != nil {
			return "", err
		}
	}

	destinationPath := path.Join(tmpFolderPath, "projectCode.zip")

	if err := ZipDirectory(tmpFolderPath, destinationPath); err != nil {
		return "", err
	}

	return destinationPath, nil
}
