package ids

import "sync/atomic"

type DeviceID uint64
type FileID uint64
type TagID uint64
type ScopeID uint64

type Generator struct {
	deviceCounter uint64
	fileCounter   uint64
	tagCounter    uint64
	scopeCounter  uint64
}

func NewGenerator() *Generator {
	return &Generator{
		deviceCounter: 0,
		fileCounter:   0,
		tagCounter:    0,
		scopeCounter:  0,
	}
}

func (g *Generator) NextDevice() DeviceID {
	return DeviceID(atomic.AddUint64(&g.deviceCounter, 1))
}

func (g *Generator) NextFile() FileID {
	return FileID(atomic.AddUint64(&g.fileCounter, 1))
}

func (g *Generator) NextTag() TagID {
	return TagID(atomic.AddUint64(&g.tagCounter, 1))
}

func (g *Generator) NextScope() ScopeID {
	return ScopeID(atomic.AddUint64(&g.scopeCounter, 1))
}

type GeneratorSnapshot struct {
	DeviceCounter uint64
	FileCounter   uint64
	TagCounter    uint64
	ScopeCounter  uint64
}

func (g *Generator) Snapshot() GeneratorSnapshot {
	return GeneratorSnapshot{
		DeviceCounter: atomic.LoadUint64(&g.deviceCounter),
		FileCounter:   atomic.LoadUint64(&g.fileCounter),
		TagCounter:    atomic.LoadUint64(&g.tagCounter),
		ScopeCounter:  atomic.LoadUint64(&g.scopeCounter),
	}
}

func (g *Generator) Restore(snap GeneratorSnapshot) {
	atomic.StoreUint64(&g.deviceCounter, snap.DeviceCounter)
	atomic.StoreUint64(&g.fileCounter, snap.FileCounter)
	atomic.StoreUint64(&g.tagCounter, snap.TagCounter)
	atomic.StoreUint64(&g.scopeCounter, snap.ScopeCounter)
}
