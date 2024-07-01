package utils

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func ZipDirectory(srcToZip, dstToZip string) error {
	destinationFile, err := os.Create(dstToZip)
	if err != nil {
		return err
	}

	var exclusionsList []string
	for _, exclusion := range ExcludedFiles {
		files, err := filepath.Glob(srcToZip + "/" + exclusion)
		if err != nil {
			return err
		}

		exclusionsList = append(exclusionsList, files...)
	}

	myZip := zip.NewWriter(destinationFile)
	err = filepath.Walk(srcToZip, func(filePath string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if err != nil {
			return err
		}
		if slices.Contains(exclusionsList, filePath) {
			return nil
		}
		relPath := strings.TrimPrefix(filePath, srcToZip)
		zipFile, err := myZip.Create(relPath)
		if err != nil {
			return err
		}
		fsFile, err := os.Open(filePath)
		if err != nil {
			return err
		}
		_, err = io.Copy(zipFile, fsFile)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	err = myZip.Close()
	if err != nil {
		return err
	}
	return nil
}
