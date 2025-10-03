package main

// Catalog represents the file metadata catalog
type Catalog struct {
	entries map[string]Metadata
}

// Metadata contains the information of each stored file
type Metadata struct {
	BlobId   string   
	FileName string
	Tags     []string
}
