package storage

type PageID uint64

type Store interface {
	Alloc(n int) []PageID
	Free(pid PageID)
	Read(pid PageID, off, n int) ([]byte, error)
	Write(pid PageID, off int, data []byte) (int, error)
	Close() error

	SaveMetadata(key string, data []byte) error
	LoadMetadata(key string) ([]byte, error)
}
