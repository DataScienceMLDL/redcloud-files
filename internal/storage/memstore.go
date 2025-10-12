package storage

import (
	"fmt"
	"sync"

	"github.com/sekai02/redcloud-files/internal/sys"
)

type MemStore struct {
	mu       sync.RWMutex
	pages    map[PageID][]byte
	nextID   PageID
	freeList []PageID
}

func NewMemStore() *MemStore {
	return &MemStore{
		pages:    make(map[PageID][]byte),
		nextID:   1,
		freeList: []PageID{},
	}
}

func (s *MemStore) Alloc(n int) []PageID {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]PageID, n)
	for i := 0; i < n; i++ {
		var pid PageID
		if len(s.freeList) > 0 {
			pid = s.freeList[len(s.freeList)-1]
			s.freeList = s.freeList[:len(s.freeList)-1]
		} else {
			pid = s.nextID
			s.nextID++
		}
		s.pages[pid] = make([]byte, sys.PageSize)
		result[i] = pid
	}
	return result
}

func (s *MemStore) Free(pid PageID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.pages[pid]; exists {
		delete(s.pages, pid)
		s.freeList = append(s.freeList, pid)
	}
}

func (s *MemStore) Read(pid PageID, off, n int) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	page, exists := s.pages[pid]
	if !exists {
		return nil, fmt.Errorf("page %d not found", pid)
	}

	if off < 0 || off >= sys.PageSize {
		return nil, fmt.Errorf("offset %d out of bounds", off)
	}

	end := off + n
	if end > sys.PageSize {
		end = sys.PageSize
	}

	result := make([]byte, end-off)
	copy(result, page[off:end])
	return result, nil
}

func (s *MemStore) Write(pid PageID, off int, data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	page, exists := s.pages[pid]
	if !exists {
		return 0, fmt.Errorf("page %d not found", pid)
	}

	if off < 0 || off >= sys.PageSize {
		return 0, fmt.Errorf("offset %d out of bounds", off)
	}

	available := sys.PageSize - off
	toCopy := len(data)
	if toCopy > available {
		toCopy = available
	}

	copy(page[off:], data[:toCopy])
	return toCopy, nil
}

func (s *MemStore) SaveMetadata(key string, data []byte) error {
	return nil
}

func (s *MemStore) LoadMetadata(key string) ([]byte, error) {
	return nil, fmt.Errorf("metadata not found in memory store")
}
