package utils

import (
	"build-machine/dtos"
	"encoding/base64"
	"log"
	"os"
	"path"
	"strings"
)

func WriteCodeMapToDirAndZip(code map[string]dtos.File, tmpFolderPath string) (string, error) {
	// Write code to temp folder
	for fileName, file := range code {
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

		fileBytes := []byte{}
        if (file.IsBase64Encoded) {
            decoded, err := base64.StdEncoding.DecodeString(file.Content)
            if err != nil {
                return "", err
            }
            fileBytes = decoded
        } else {
            fileBytes = []byte(file.Content)
        }

        err := os.WriteFile(filePath, fileBytes, 0644)
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
