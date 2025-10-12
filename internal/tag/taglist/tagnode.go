package taglist

import (
	"fmt"
	"sync"

	"github.com/sekai02/redcloud-files/internal/ids"
	"github.com/sekai02/redcloud-files/internal/sys"
)

type TagNode struct {
	NameFixed  [sys.TagNameMaxLen]byte
	FilesCount uint32
	FixedFIDs  [sys.FixedFIDsCount]ids.FileID
	Overflow   []ids.FileID
}

type List struct {
	mu       sync.RWMutex
	tags     map[ids.TagID]*TagNode
	nameToID map[string]ids.TagID
	idGen    *ids.Generator
	freeList []ids.TagID
}

func NewList(idGen *ids.Generator) *List {
	return &List{
		tags:     make(map[ids.TagID]*TagNode),
		nameToID: make(map[string]ids.TagID),
		idGen:    idGen,
		freeList: []ids.TagID{},
	}
}

func encodeTagName(name string) [sys.TagNameMaxLen]byte {
	var fixed [sys.TagNameMaxLen]byte
	copy(fixed[:], name)
	return fixed
}

func decodeTagName(fixed [sys.TagNameMaxLen]byte) string {
	end := 0
	for i, b := range fixed {
		if b == 0 {
			break
		}
		end = i + 1
	}
	return string(fixed[:end])
}

func (l *List) AllocTagID(name string) (ids.TagID, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if tid, exists := l.nameToID[name]; exists {
		return tid, false
	}

	var tid ids.TagID
	if len(l.freeList) > 0 {
		tid = l.freeList[len(l.freeList)-1]
		l.freeList = l.freeList[:len(l.freeList)-1]
	} else {
		tid = l.idGen.NextTag()
	}

	node := &TagNode{
		NameFixed:  encodeTagName(name),
		FilesCount: 0,
		FixedFIDs:  [sys.FixedFIDsCount]ids.FileID{},
		Overflow:   []ids.FileID{},
	}

	l.tags[tid] = node
	l.nameToID[name] = tid
	return tid, true
}

func (l *List) FindTagID(name string) (ids.TagID, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	tid, exists := l.nameToID[name]
	return tid, exists
}

func (l *List) AddFID(tid ids.TagID, fid ids.FileID) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	node, exists := l.tags[tid]
	if !exists {
		return fmt.Errorf("tag %d not found", tid)
	}

	if node.FilesCount < sys.FixedFIDsCount {
		node.FixedFIDs[node.FilesCount] = fid
	} else {
		node.Overflow = append(node.Overflow, fid)
	}
	node.FilesCount++

	return nil
}

func (l *List) RemoveFID(tid ids.TagID, fid ids.FileID) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	node, exists := l.tags[tid]
	if !exists {
		return fmt.Errorf("tag %d not found", tid)
	}

	for i := uint32(0); i < node.FilesCount && i < sys.FixedFIDsCount; i++ {
		if node.FixedFIDs[i] == fid {
			for j := i; j < node.FilesCount-1 && j < sys.FixedFIDsCount-1; j++ {
				node.FixedFIDs[j] = node.FixedFIDs[j+1]
			}
			if node.FilesCount > sys.FixedFIDsCount && len(node.Overflow) > 0 {
				node.FixedFIDs[sys.FixedFIDsCount-1] = node.Overflow[0]
				node.Overflow = node.Overflow[1:]
			}
			node.FilesCount--
			if node.FilesCount == 0 {
				l.releaseTag(tid, node)
			}
			return nil
		}
	}

	for i, overflowFID := range node.Overflow {
		if overflowFID == fid {
			node.Overflow = append(node.Overflow[:i], node.Overflow[i+1:]...)
			node.FilesCount--
			if node.FilesCount == 0 {
				l.releaseTag(tid, node)
			}
			return nil
		}
	}

	return fmt.Errorf("file %d not found in tag %d", fid, tid)
}

func (l *List) releaseTag(tid ids.TagID, node *TagNode) {
	name := decodeTagName(node.NameFixed)
	delete(l.tags, tid)
	delete(l.nameToID, name)
	l.freeList = append(l.freeList, tid)
}

func (l *List) TagName(tid ids.TagID) string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	node, exists := l.tags[tid]
	if !exists {
		return ""
	}

	return decodeTagName(node.NameFixed)
}

func (l *List) Files(tid ids.TagID) []ids.FileID {
	l.mu.RLock()
	defer l.mu.RUnlock()

	node, exists := l.tags[tid]
	if !exists {
		return []ids.FileID{}
	}

	result := make([]ids.FileID, 0, node.FilesCount)
	for i := uint32(0); i < node.FilesCount && i < sys.FixedFIDsCount; i++ {
		result = append(result, node.FixedFIDs[i])
	}
	result = append(result, node.Overflow...)

	return result
}

type TagListSnapshot struct {
	Tags     map[ids.TagID]*TagNode
	NameToID map[string]ids.TagID
	FreeList []ids.TagID
}

func (l *List) Snapshot() TagListSnapshot {
	l.mu.RLock()
	defer l.mu.RUnlock()

	tagsCopy := make(map[ids.TagID]*TagNode)
	for k, v := range l.tags {
		nodeCopy := &TagNode{
			NameFixed:  v.NameFixed,
			FilesCount: v.FilesCount,
			FixedFIDs:  v.FixedFIDs,
			Overflow:   append([]ids.FileID{}, v.Overflow...),
		}
		tagsCopy[k] = nodeCopy
	}

	nameToIDCopy := make(map[string]ids.TagID)
	for k, v := range l.nameToID {
		nameToIDCopy[k] = v
	}

	freeListCopy := append([]ids.TagID{}, l.freeList...)

	return TagListSnapshot{
		Tags:     tagsCopy,
		NameToID: nameToIDCopy,
		FreeList: freeListCopy,
	}
}

func (l *List) Restore(snap TagListSnapshot) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.tags = snap.Tags
	l.nameToID = snap.NameToID
	l.freeList = snap.FreeList
}
