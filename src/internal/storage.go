package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func StoreBlob(filePath string, storeDir string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	blobID := hex.EncodeToString(hash.Sum(nil))

	destPath := filepath.Join(storeDir, blobID)

	if _, err := os.Stat(destPath); err == nil {
		return blobID, nil
	}

	file.Seek(0, 0)
	destFile, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, file); err != nil {
		return "", err
	}

	return blobID, nil
}

func deleteBlobFile(blobID, storeDir string) error {
	blobPath := filepath.Join(storeDir, blobID)
	err := os.Remove(blobPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func AddFiles(fileList []string, tags []string, storeDir string, catalog *Catalog) error {
	for _, filePath := range fileList {
		blobID, err := StoreBlob(filePath, storeDir)
		if err != nil {
			return fmt.Errorf("error guardando %s: %w", filePath, err)
		}

		filename := filepath.Base(filePath)
		catalog.AddEntry(blobID, filename, tags)

		fmt.Printf("Archivo %s guardado como blob %s con etiquetas %v\n", filename, blobID, tags)
	}
	return nil
}
