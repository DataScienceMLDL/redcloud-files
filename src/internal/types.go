package main

type FileID string
type TagID int

type FileNode struct {
	Id      FileID
	BlobId  string
	FileName string
	Tags    []TagID
}

type TagNode struct {
	Id      TagID
	Name    string
	FileIDs []FileID
}

type Catalog struct {
	files map[FileID]*FileNode
	tags  map[TagID]*TagNode
	nextTagId TagID
}