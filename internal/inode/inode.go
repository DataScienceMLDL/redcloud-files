package inode

import (
	"context"
	"fmt"
	"sync"

	"github.com/sekai02/redcloud-files/internal/ids"
	"github.com/sekai02/redcloud-files/internal/storage"
	"github.com/sekai02/redcloud-files/internal/sys"
)

type Inode struct {
	Size      int64
	DataPages []storage.PageID
	TagIDs    []ids.TagID
}

type Manager struct {
	mu     sync.RWMutex
	inodes map[ids.DeviceID]map[ids.FileID]*Inode
	store  storage.Store
	idGen  *ids.Generator
}

func NewManager(store storage.Store, idGen *ids.Generator) *Manager {
	return &Manager{
		inodes: make(map[ids.DeviceID]map[ids.FileID]*Inode),
		store:  store,
		idGen:  idGen,
	}
}

func (m *Manager) Create(ctx context.Context, dev ids.DeviceID) (ids.FileID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.inodes[dev]; !exists {
		m.inodes[dev] = make(map[ids.FileID]*Inode)
	}

	fid := m.idGen.NextFile()
	m.inodes[dev][fid] = &Inode{
		Size:      0,
		DataPages: []storage.PageID{},
		TagIDs:    []ids.TagID{},
	}

	return fid, nil
}

func (m *Manager) Delete(ctx context.Context, dev ids.DeviceID, fid ids.FileID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	devInodes, exists := m.inodes[dev]
	if !exists {
		return fmt.Errorf("device %d not found", dev)
	}

	inode, exists := devInodes[fid]
	if !exists {
		return fmt.Errorf("file %d not found on device %d", fid, dev)
	}

	for _, pid := range inode.DataPages {
		m.store.Free(pid)
	}

	delete(devInodes, fid)
	return nil
}

func (m *Manager) Read(ctx context.Context, dev ids.DeviceID, fid ids.FileID, off, n int64) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devInodes, exists := m.inodes[dev]
	if !exists {
		return nil, fmt.Errorf("device %d not found", dev)
	}

	inode, exists := devInodes[fid]
	if !exists {
		return nil, fmt.Errorf("file %d not found on device %d", fid, dev)
	}

	if off < 0 || off >= inode.Size {
		return []byte{}, nil
	}

	if off+n > inode.Size {
		n = inode.Size - off
	}

	result := make([]byte, 0, n)
	pageIdx := int(off / int64(sys.PageSize))
	pageOff := int(off % int64(sys.PageSize))
	remaining := int(n)

	for remaining > 0 && pageIdx < len(inode.DataPages) {
		toRead := sys.PageSize - pageOff
		if toRead > remaining {
			toRead = remaining
		}

		data, err := m.store.Read(inode.DataPages[pageIdx], pageOff, toRead)
		if err != nil {
			return nil, fmt.Errorf("read page %d: %w", inode.DataPages[pageIdx], err)
		}

		result = append(result, data...)
		remaining -= len(data)
		pageIdx++
		pageOff = 0
	}

	return result, nil
}

func (m *Manager) Write(ctx context.Context, dev ids.DeviceID, fid ids.FileID, off int64, p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	devInodes, exists := m.inodes[dev]
	if !exists {
		return 0, fmt.Errorf("device %d not found", dev)
	}

	inode, exists := devInodes[fid]
	if !exists {
		return 0, fmt.Errorf("file %d not found on device %d", fid, dev)
	}

	endPos := off + int64(len(p))
	requiredPages := (endPos + int64(sys.PageSize) - 1) / int64(sys.PageSize)

	for int64(len(inode.DataPages)) < requiredPages {
		newPages := m.store.Alloc(1)
		inode.DataPages = append(inode.DataPages, newPages[0])
	}

	written := 0
	pageIdx := int(off / int64(sys.PageSize))
	pageOff := int(off % int64(sys.PageSize))
	remaining := len(p)

	for remaining > 0 && pageIdx < len(inode.DataPages) {
		toWrite := sys.PageSize - pageOff
		if toWrite > remaining {
			toWrite = remaining
		}

		n, err := m.store.Write(inode.DataPages[pageIdx], pageOff, p[written:written+toWrite])
		if err != nil {
			return written, fmt.Errorf("write page %d: %w", inode.DataPages[pageIdx], err)
		}

		written += n
		remaining -= n
		pageIdx++
		pageOff = 0
	}

	if endPos > inode.Size {
		inode.Size = endPos
	}

	return written, nil
}

func (m *Manager) TagIDs(ctx context.Context, dev ids.DeviceID, fid ids.FileID) ([]ids.TagID, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devInodes, exists := m.inodes[dev]
	if !exists {
		return nil, fmt.Errorf("device %d not found", dev)
	}

	inode, exists := devInodes[fid]
	if !exists {
		return nil, fmt.Errorf("file %d not found on device %d", fid, dev)
	}

	result := make([]ids.TagID, len(inode.TagIDs))
	copy(result, inode.TagIDs)
	return result, nil
}

func (m *Manager) AddTagID(ctx context.Context, dev ids.DeviceID, fid ids.FileID, tid ids.TagID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	devInodes, exists := m.inodes[dev]
	if !exists {
		return fmt.Errorf("device %d not found", dev)
	}

	inode, exists := devInodes[fid]
	if !exists {
		return fmt.Errorf("file %d not found on device %d", fid, dev)
	}

	for _, existingTID := range inode.TagIDs {
		if existingTID == tid {
			return nil
		}
	}

	inode.TagIDs = append(inode.TagIDs, tid)
	return nil
}

func (m *Manager) RemoveTagID(ctx context.Context, dev ids.DeviceID, fid ids.FileID, tid ids.TagID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	devInodes, exists := m.inodes[dev]
	if !exists {
		return fmt.Errorf("device %d not found", dev)
	}

	inode, exists := devInodes[fid]
	if !exists {
		return fmt.Errorf("file %d not found on device %d", fid, dev)
	}

	for i, existingTID := range inode.TagIDs {
		if existingTID == tid {
			inode.TagIDs = append(inode.TagIDs[:i], inode.TagIDs[i+1:]...)
			return nil
		}
	}

	return nil
}

func (m *Manager) Exists(ctx context.Context, dev ids.DeviceID, fid ids.FileID) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devInodes, exists := m.inodes[dev]
	if !exists {
		return false
	}

	_, exists = devInodes[fid]
	return exists
}

func (m *Manager) GetStore() storage.Store {
	return m.store
}

func (m *Manager) GetIDGen() *ids.Generator {
	return m.idGen
}

type InodeData struct {
	Size      int64
	DataPages []storage.PageID
	TagIDs    []ids.TagID
}

func (m *Manager) SnapshotInodes() map[ids.DeviceID]map[ids.FileID]InodeData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[ids.DeviceID]map[ids.FileID]InodeData)
	for dev, devInodes := range m.inodes {
		result[dev] = make(map[ids.FileID]InodeData)
		for fid, inode := range devInodes {
			result[dev][fid] = InodeData{
				Size:      inode.Size,
				DataPages: append([]storage.PageID{}, inode.DataPages...),
				TagIDs:    append([]ids.TagID{}, inode.TagIDs...),
			}
		}
	}
	return result
}

func (m *Manager) RestoreInodes(data map[ids.DeviceID]map[ids.FileID]InodeData) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.inodes = make(map[ids.DeviceID]map[ids.FileID]*Inode)
	for dev, devInodes := range data {
		m.inodes[dev] = make(map[ids.FileID]*Inode)
		for fid, inodeData := range devInodes {
			m.inodes[dev][fid] = &Inode{
				Size:      inodeData.Size,
				DataPages: inodeData.DataPages,
				TagIDs:    inodeData.TagIDs,
			}
		}
	}
}
