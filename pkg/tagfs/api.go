package tagfs

import (
	"context"

	"github.com/sekai02/redcloud-files/internal/ids"
)

type FileTagAPI interface {
	Create(ctx context.Context, deviceID uint64) (uint64, error)
	Copy(ctx context.Context, deviceID, fileID, destinationDeviceID uint64) (uint64, error)
	Delete(ctx context.Context, deviceID, fileID uint64) error
	Read(ctx context.Context, deviceID, fileID uint64, offset, length int64) ([]byte, error)
	Write(ctx context.Context, deviceID, fileID uint64, offset int64, data []byte) (int, error)
	TagAdd(ctx context.Context, deviceID, fileID uint64, tagName string) error
	TagRemove(ctx context.Context, deviceID, fileID uint64, tagName string) error
	TagList(ctx context.Context, deviceID, fileID uint64) ([]string, error)
	DeviceList(ctx context.Context) ([]uint64, error)
}

type ScopeAPI interface {
	MkScope(ctx context.Context) (uint64, error)
	ScopeAddSource(ctx context.Context, scopeID, sourceID uint64) error
	ScopeRmSource(ctx context.Context, scopeID, sourceID uint64) error
	ScopeAddFilter(ctx context.Context, scopeID uint64, tags ...string) error
	ScopeRmFilter(ctx context.Context, scopeID uint64, tags ...string) error
	List(ctx context.Context, scopeID uint64) ([][2]uint64, error)
}

type API interface {
	FileTagAPI
	ScopeAPI
}

func DeviceIDFromUint64(v uint64) ids.DeviceID {
	return ids.DeviceID(v)
}

func FileIDFromUint64(v uint64) ids.FileID {
	return ids.FileID(v)
}

func ScopeIDFromUint64(v uint64) ids.ScopeID {
	return ids.ScopeID(v)
}

func DeviceIDToUint64(v ids.DeviceID) uint64 {
	return uint64(v)
}

func FileIDToUint64(v ids.FileID) uint64 {
	return uint64(v)
}

func ScopeIDToUint64(v ids.ScopeID) uint64 {
	return uint64(v)
}
