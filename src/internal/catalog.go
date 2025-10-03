package main


func NewCatalog() *Catalog {
	return &Catalog{
		files:     make(map[FileID]*FileNode),
		tags:      make(map[TagID]*TagNode),
		nextTagId: 1,
	}
}

func (c *Catalog) findTagByName(name string) (*TagNode, bool) {
	for _, tag := range c.tags {
		if tag.Name == name {
			return tag, true
		}
	}
	return nil, false
}

func (c *Catalog) createTag(name string) *TagNode {
	tag := &TagNode{
		Id:      c.nextTagId,
		Name:    name,
		FileIDs: []FileID{},
	}
	c.tags[c.nextTagId] = tag
	c.nextTagId++
	return tag
}

func (c *Catalog) AddEntry(blobId, filename string, tagNames []string) {
	fileID := FileID(blobId)
	if _, exists := c.files[fileID]; exists {
		return 
	}

	fileNode := &FileNode{
		Id:       fileID,
		BlobId:   blobId,
		FileName: filename,
		Tags:     []TagID{},
	}

	for _, name := range tagNames {
		tag, ok := c.findTagByName(name)
		if !ok {
			tag = c.createTag(name)
		}
		fileNode.Tags = append(fileNode.Tags, tag.Id)
		tag.FileIDs = append(tag.FileIDs, fileID)
	}

	c.files[fileID] = fileNode
}

func (c *Catalog) AddTag(blobId, tagName string) {
	fileID := FileID(blobId)
	file, exists := c.files[fileID]
	if !exists {
		return
	}

	tag, ok := c.findTagByName(tagName)
	if !ok {
		tag = c.createTag(tagName)
	}

	for _, tid := range file.Tags {
		if tid == tag.Id {
			return
		}
	}

	file.Tags = append(file.Tags, tag.Id)
	tag.FileIDs = append(tag.FileIDs, fileID)
}

func (c *Catalog) RemoveTag(blobId, tagName string) {
	fileID := FileID(blobId)
	file, exists := c.files[fileID]
	if !exists {
		return
	}

	tag, ok := c.findTagByName(tagName)
	if !ok {
		return
	}

	newTags := []TagID{}
	for _, tid := range file.Tags {
		if tid != tag.Id {
			newTags = append(newTags, tid)
		}
	}
	file.Tags = newTags

	newFiles := []FileID{}
	for _, fid := range tag.FileIDs {
		if fid != fileID {
			newFiles = append(newFiles, fid)
		}
	}
	tag.FileIDs = newFiles
}

func (c *Catalog) FindByTag(tagName string) []*FileNode {
	tag, ok := c.findTagByName(tagName)
	if !ok {
		return nil
	}

	results := []*FileNode{}
	for _, fid := range tag.FileIDs {
		if file, ok := c.files[fid]; ok {
			results = append(results, file)
		}
	}
	return results
}

func (c *Catalog) FindByTags(tagNames []string) []*FileNode {
	results := []*FileNode{}
	seen := make(map[FileID]bool)

	for _, name := range tagNames {
		tag, ok := c.findTagByName(name)
		if !ok {
			continue
		}
		for _, fid := range tag.FileIDs {
			if !seen[fid] {
				results = append(results, c.files[fid])
				seen[fid] = true
			}
		}
	}
	return results
}

func (c *Catalog) TagNamesOf(file *FileNode) []string {
    names := make([]string, 0, len(file.Tags))
    for _, tid := range file.Tags {
        if tag, ok := c.tags[tid]; ok {
            names = append(names, tag.Name)
        }
    }
    return names
}

func (c *Catalog) DeleteByTags(tagNames []string, storeDir string) ([]*FileNode, error) {
	toDelete := make(map[FileID]*FileNode)

	for _, name := range tagNames {
		tag, ok := c.findTagByName(name)
		if !ok {
			continue
		}
		for _, fid := range tag.FileIDs {
			if file, ok := c.files[fid]; ok {
				toDelete[fid] = file
			}
		}
	}

	deletedFiles := []*FileNode{}
	for fid, file := range toDelete {
		if err := deleteBlobFile(file.BlobId, storeDir); err != nil {
			return nil, err
		}
		delete(c.files, fid)
		deletedFiles = append(deletedFiles, file)
	}

	for _, tag := range c.tags {
		newFileIDs := []FileID{}
		for _, fid := range tag.FileIDs {
			if _, marked := toDelete[fid]; !marked {
				newFileIDs = append(newFileIDs, fid)
			}
		}
		tag.FileIDs = newFileIDs
	}

	return deletedFiles, nil
}

func (c *Catalog) Count() int {
	return len(c.files)
}

func (c *Catalog) GetAllEntries() map[string]*FileNode {
	result := make(map[string]*FileNode)
	for _, file := range c.files {
		result[file.BlobId] = file
	}
	return result
}
