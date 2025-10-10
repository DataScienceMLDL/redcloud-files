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
    if len(tagNames) == 0 {
        return nil
    }

    // Normaliza nombres (quita duplicados) y resuelve TagNodes
    uniq := make(map[string]struct{})
    tagNodes := make([]*TagNode, 0, len(tagNames))

    for _, name := range tagNames {
        if _, dup := uniq[name]; dup {
            continue
        }
        tag, ok := c.findTagByName(name)
        if !ok {
            // Si falta alguno, no hay archivos que tengan TODOS
            return []*FileNode{}
        }
        uniq[name] = struct{}{}
        tagNodes = append(tagNodes, tag)
    }

    // Intersección de FileIDs (AND) — permite tags extra
    if len(tagNodes) == 0 {
        return []*FileNode{}
    }
    candidates := make(map[FileID]bool)
    for _, fid := range tagNodes[0].FileIDs {
        candidates[fid] = true
    }
    for _, t := range tagNodes[1:] {
        next := make(map[FileID]bool)
        for _, fid := range t.FileIDs {
            if candidates[fid] {
                next[fid] = true
            }
        }
        candidates = next
        if len(candidates) == 0 {
            return []*FileNode{}
        }
    }

    results := make([]*FileNode, 0, len(candidates))
    for fid := range candidates {
        if file, ok := c.files[fid]; ok {
            results = append(results, file)
        }
    }
    return results
}

func (c *Catalog) FindByTagsExact(tagNames []string) []*FileNode {
    if len(tagNames) == 0 {
        return nil
    }

    uniq := make(map[string]struct{})
    tagNodes := make([]*TagNode, 0, len(tagNames))
    tagIDSet := make(map[TagID]struct{}, len(tagNames))

    for _, name := range tagNames {
        if _, dup := uniq[name]; dup {
            continue
        }
        tag, ok := c.findTagByName(name)
        if !ok {
            return []*FileNode{}
        }
        uniq[name] = struct{}{}
        tagNodes = append(tagNodes, tag)
        tagIDSet[tag.Id] = struct{}{}
    }

    if len(tagNodes) == 0 {
        return []*FileNode{}
    }
    candidates := make(map[FileID]bool)
    for _, fid := range tagNodes[0].FileIDs {
        candidates[fid] = true
    }
    for _, t := range tagNodes[1:] {
        next := make(map[FileID]bool)
        for _, fid := range t.FileIDs {
            if candidates[fid] {
                next[fid] = true
            }
        }
        candidates = next
        if len(candidates) == 0 {
            return []*FileNode{}
        }
    }

    results := []*FileNode{}
    for fid := range candidates {
        file, ok := c.files[fid]
        if !ok {
            continue
        }
        if len(file.Tags) != len(tagIDSet) {
            continue
        }
        match := true
        for _, tid := range file.Tags {
            if _, ok := tagIDSet[tid]; !ok {
                match = false
                break
            }
        }
        if match {
            results = append(results, file)
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
    matches := c.FindByTagsExact(tagNames)
    if len(matches) == 0 {
        return nil, nil
    }

    toDelete := make(map[FileID]*FileNode, len(matches))
    for _, f := range matches {
        toDelete[f.Id] = f
    }

    deletedFiles := make([]*FileNode, 0, len(matches))
    for fid, file := range toDelete {
        if err := deleteBlobFile(file.BlobId, storeDir); err != nil {
            return nil, err
        }
        delete(c.files, fid)
        deletedFiles = append(deletedFiles, file)
    }

    for _, tag := range c.tags {
        if len(tag.FileIDs) == 0 {
            continue
        }
        kept := tag.FileIDs[:0]
        for _, fid := range tag.FileIDs {
            if _, marked := toDelete[fid]; !marked {
                kept = append(kept, fid)
            }
        }
        tag.FileIDs = kept
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
