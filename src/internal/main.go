package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)


// StoreBlob stores a file as a blob in the specified directory using its SHA-256 hash as the filename.
// It returns the blob ID (the hex-encoded hash) and an error, if any.
// If the blob already exists in the store directory, it does not overwrite it.
// Parameters:
//   - filePath: the path to the source file to be stored.
//   - storeDir: the directory where the blob should be stored.
// Returns:
//   - string: the blob ID (SHA-256 hash of the file contents).
//   - error: any error encountered during the operation.
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

func main() {
	storeDir := "./store"
	os.MkdirAll(storeDir, 0755)

    // Example file to store
	filePath := "example.txt"

	blobID, err := StoreBlob(filePath, storeDir)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("File stored as:", blobID)
	fmt.Println("Path in store:", filepath.Join(storeDir, blobID))
}
