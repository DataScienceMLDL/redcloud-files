package main

import "fmt"

// NewCatalog creates and returns a new Catalog instance with an initialized entries map.
func NewCatalog() *Catalog {
	return &Catalog{entries: make(map[string]Metadata)}
}

// Catalog methods
func (c *Catalog) AddEntry(blobId, filename string, tags []string) {
	c.entries[blobId] = Metadata{
		BlobId:   blobId,
		FileName: filename,
		Tags:     tags,
	}
}

func (c *Catalog) AddTag(blobId, tag string) {
	entry, exists := c.entries[blobId]
	if !exists {
		return
	}
	entry.Tags = append(entry.Tags, tag)
	c.entries[blobId] = entry
}

func (c *Catalog) RemoveTag(blobId, tag string) {
	entry, exists := c.entries[blobId]
	if !exists {
		return
	}
	var updatedTags []string
	for _, t := range entry.Tags {
		if t != tag {
			updatedTags = append(updatedTags, t)
		}
	}
	entry.Tags = updatedTags
	c.entries[blobId] = entry
}

func (c *Catalog) FindByTag(tag string) []Metadata {
	var results []Metadata
	for _, entry := range c.entries {
		for _, t := range entry.Tags {
			if t == tag {
				results = append(results, entry)
				break
			}
		}
	}
	return results
}

func (c *Catalog) FindByTags(query []string) []Metadata {
	var results []Metadata
	for _, entry := range c.entries {
		if containsAny(entry.Tags, query) {
			results = append(results, entry)
		}
	}
	return results
}

func (c *Catalog) DeleteByTags(query []string, storeDir string) ([]Metadata, error) {
	var deleted []Metadata

	for blobID, entry := range c.entries {
		if containsAll(entry.Tags, query) {
			if err := deleteBlobFile(blobID, storeDir); err != nil {
				return deleted, fmt.Errorf("no se pudo eliminar blob %s: %w", blobID, err)
			}

			delete(c.entries, blobID)
			deleted = append(deleted, entry)
		}
	}

	return deleted, nil
}

func (c *Catalog) GetAllEntries() map[string]Metadata {
	return c.entries
}

func (c *Catalog) Count() int {
	return len(c.entries)
}

// Helper functions

func containsAny(target []string, query []string) bool {
	for _, q := range query {
		for _, t := range target {
			if q == t {
				return true
			}
		}
	}
	return false
}

func containsAll(target []string, query []string) bool {
	for _, q := range query {
		found := false
		for _, t := range target {
			if q == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
