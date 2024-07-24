package utils

import (
	"archive/tar"
	"archive/zip"
	"fmt"
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
	defer destinationFile.Close()

	var exclusionsList []string
	for _, exclusion := range ExcludedFiles {
		files, err := filepath.Glob(srcToZip + "/" + exclusion)
		if err != nil {
			return err
		}

		exclusionsList = append(exclusionsList, files...)
	}

	myZip := zip.NewWriter(destinationFile)
	defer myZip.Close()
	err = filepath.Walk(srcToZip, func(filePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Create the relative path for the file
		relPath, err := filepath.Rel(srcToZip, filePath)
		if err != nil {
			return err
		}

		// Skip excluded files
		if slices.Contains(exclusionsList, filePath) {
			return nil
		}

		// If it's a directory, create it in the zip file
		if info.IsDir() {
			_, err := myZip.Create(relPath + "/")
			if err != nil {
				return err
			}
			return nil
		}

		// Otherwise, it's a file, so add it to the zip
		zipFile, err := myZip.Create(relPath)
		if err != nil {
			return err
		}

		fsFile, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer fsFile.Close() // Ensure the file is closed after usage

		_, err = io.Copy(zipFile, fsFile)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func UntarAll(reader io.Reader, destDir, prefix string) error {
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}

		if !strings.HasPrefix(header.Name, prefix) {
			return fmt.Errorf("tar contents corrupted")
		}

		mode := header.FileInfo().Mode()
		destFileName := filepath.Join(destDir, header.Name[len(prefix):])

		baseName := filepath.Dir(destFileName)
		if err := os.MkdirAll(baseName, 0755); err != nil {
			return err
		}
		if header.FileInfo().IsDir() {
			if err := os.MkdirAll(destFileName, 0755); err != nil {
				return err
			}
			continue
		}

		evaledPath, err := filepath.EvalSymlinks(baseName)
		if err != nil {
			return err
		}

		if mode&os.ModeSymlink != 0 {
			linkname := header.Linkname

			if !filepath.IsAbs(linkname) {
				_ = filepath.Join(evaledPath, linkname)
			}

			if err := os.Symlink(linkname, destFileName); err != nil {
				return err
			}
		} else {
			outFile, err := os.Create(destFileName)
			if err != nil {
				return err
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return err
			}
			if err := outFile.Close(); err != nil {
				return err
			}
		}
	}

	return nil
}
