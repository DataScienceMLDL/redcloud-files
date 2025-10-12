package storage

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v4"
	"github.com/sekai02/redcloud-files/internal/sys"
)

type BadgerStore struct {
	db       *badger.DB
	mu       sync.RWMutex
	nextID   PageID
	freeList []PageID
}

func NewBadgerStore(path string) (*BadgerStore, error) {
	opts := badger.DefaultOptions(path)
	opts.Logger = nil

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open badger: %w", err)
	}

	store := &BadgerStore{
		db:       db,
		nextID:   1,
		freeList: []PageID{},
	}

	if err := store.loadMetadata(); err != nil {
		db.Close()
		return nil, fmt.Errorf("load metadata: %w", err)
	}

	return store, nil
}

func (s *BadgerStore) loadMetadata() error {
	return s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("meta:nextID"))
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			if len(val) == 8 {
				s.nextID = PageID(binary.BigEndian.Uint64(val))
			}
			return nil
		})
	})
}

func (s *BadgerStore) saveMetadata() error {
	return s.db.Update(func(txn *badger.Txn) error {
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(s.nextID))
		return txn.Set([]byte("meta:nextID"), buf)
	})
}

func (s *BadgerStore) Alloc(n int) []PageID {
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

		key := pageKey(pid)
		page := make([]byte, sys.PageSize)

		s.db.Update(func(txn *badger.Txn) error {
			return txn.Set(key, page)
		})

		result[i] = pid
	}

	s.saveMetadata()
	return result
}

func (s *BadgerStore) Free(pid PageID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := pageKey(pid)
	s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})

	s.freeList = append(s.freeList, pid)
}

func (s *BadgerStore) Read(pid PageID, off, n int) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if off < 0 || off >= sys.PageSize {
		return nil, fmt.Errorf("offset %d out of bounds", off)
	}

	key := pageKey(pid)
	var result []byte

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return fmt.Errorf("page %d not found", pid)
		}

		return item.Value(func(val []byte) error {
			end := off + n
			if end > sys.PageSize {
				end = sys.PageSize
			}

			result = make([]byte, end-off)
			copy(result, val[off:end])
			return nil
		})
	})

	return result, err
}

func (s *BadgerStore) Write(pid PageID, off int, data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if off < 0 || off >= sys.PageSize {
		return 0, fmt.Errorf("offset %d out of bounds", off)
	}

	key := pageKey(pid)

	err := s.db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return fmt.Errorf("page %d not found", pid)
		}

		var page []byte
		err = item.Value(func(val []byte) error {
			page = make([]byte, sys.PageSize)
			copy(page, val)
			return nil
		})
		if err != nil {
			return err
		}

		available := sys.PageSize - off
		toCopy := len(data)
		if toCopy > available {
			toCopy = available
		}

		copy(page[off:], data[:toCopy])
		return txn.Set(key, page)
	})

	if err != nil {
		return 0, err
	}

	available := sys.PageSize - off
	written := len(data)
	if written > available {
		written = available
	}

	return written, nil
}

func (s *BadgerStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.saveMetadata()
	return s.db.Close()
}

func pageKey(pid PageID) []byte {
	key := make([]byte, 9)
	key[0] = 'p'
	binary.BigEndian.PutUint64(key[1:], uint64(pid))
	return key
}

func metaKey(name string) []byte {
	return []byte("meta:" + name)
}

func (s *BadgerStore) SaveMetadata(key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(metaKey(key), data)
	})
}

func (s *BadgerStore) LoadMetadata(key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(metaKey(key))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			result = make([]byte, len(val))
			copy(result, val)
			return nil
		})
	})

	return result, err
}
