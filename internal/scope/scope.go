package scope

import (
	"context"
	"fmt"
	"sync"

	"github.com/sekai02/redcloud-files/internal/ids"
	"github.com/sekai02/redcloud-files/internal/index"
)

type Scope struct {
	ID         ids.ScopeID
	Sources    map[uint64]struct{}
	Filters    map[string]struct{}
	Cache      map[[2]uint64]struct{}
	CacheValid bool
	SubScopes  map[ids.ScopeID]struct{}
}

type Manager struct {
	mu              sync.RWMutex
	scopes          map[ids.ScopeID]*Scope
	tagScopeRef     map[string]map[ids.ScopeID]struct{}
	idGen           *ids.Generator
	tagListProvider TagListProvider
}

type TagListProvider interface {
	Files(tid ids.TagID) []ids.FileID
	FindTagID(name string) (ids.TagID, bool)
}

func NewManager(idGen *ids.Generator, tagListProvider TagListProvider) *Manager {
	return &Manager{
		scopes:          make(map[ids.ScopeID]*Scope),
		tagScopeRef:     make(map[string]map[ids.ScopeID]struct{}),
		idGen:           idGen,
		tagListProvider: tagListProvider,
	}
}

func (m *Manager) MkScope(ctx context.Context) (ids.ScopeID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sid := m.idGen.NextScope()
	m.scopes[sid] = &Scope{
		ID:         sid,
		Sources:    make(map[uint64]struct{}),
		Filters:    make(map[string]struct{}),
		Cache:      make(map[[2]uint64]struct{}),
		CacheValid: false,
		SubScopes:  make(map[ids.ScopeID]struct{}),
	}

	return sid, nil
}

func (m *Manager) AddSource(ctx context.Context, sid ids.ScopeID, sourceID uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	scope, exists := m.scopes[sid]
	if !exists {
		return fmt.Errorf("scope %d not found", sid)
	}

	if sourceID >= uint64(1<<32) {
		subScopeID := ids.ScopeID(sourceID)
		if subScopeID == sid {
			return fmt.Errorf("scope cannot reference itself")
		}
		if m.hasCycle(sid, subScopeID) {
			return fmt.Errorf("adding source would create a cycle")
		}
		if subScope, exists := m.scopes[subScopeID]; exists {
			subScope.SubScopes[sid] = struct{}{}
		}
	}

	scope.Sources[sourceID] = struct{}{}
	m.invalidateScope(scope)

	return nil
}

func (m *Manager) RmSource(ctx context.Context, sid ids.ScopeID, sourceID uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	scope, exists := m.scopes[sid]
	if !exists {
		return fmt.Errorf("scope %d not found", sid)
	}

	delete(scope.Sources, sourceID)

	if sourceID >= uint64(1<<32) {
		subScopeID := ids.ScopeID(sourceID)
		if subScope, exists := m.scopes[subScopeID]; exists {
			delete(subScope.SubScopes, sid)
		}
	}

	m.invalidateScope(scope)

	return nil
}

func (m *Manager) AddFilter(ctx context.Context, sid ids.ScopeID, tags ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	scope, exists := m.scopes[sid]
	if !exists {
		return fmt.Errorf("scope %d not found", sid)
	}

	for _, tag := range tags {
		scope.Filters[tag] = struct{}{}

		if _, exists := m.tagScopeRef[tag]; !exists {
			m.tagScopeRef[tag] = make(map[ids.ScopeID]struct{})
		}
		m.tagScopeRef[tag][sid] = struct{}{}
	}

	m.invalidateScope(scope)

	return nil
}

func (m *Manager) RmFilter(ctx context.Context, sid ids.ScopeID, tags ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	scope, exists := m.scopes[sid]
	if !exists {
		return fmt.Errorf("scope %d not found", sid)
	}

	for _, tag := range tags {
		delete(scope.Filters, tag)

		if scopeSet, exists := m.tagScopeRef[tag]; exists {
			delete(scopeSet, sid)
			if len(scopeSet) == 0 {
				delete(m.tagScopeRef, tag)
			}
		}
	}

	m.invalidateScope(scope)

	return nil
}

func (m *Manager) List(ctx context.Context, sid ids.ScopeID) ([][2]uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	scope, exists := m.scopes[sid]
	if !exists {
		return nil, fmt.Errorf("scope %d not found", sid)
	}

	if scope.CacheValid {
		result := make([][2]uint64, 0, len(scope.Cache))
		for pair := range scope.Cache {
			result = append(result, pair)
		}
		return result, nil
	}

	resultSet := make(map[[2]uint64]struct{})

	for sourceID := range scope.Sources {
		if sourceID < uint64(1<<32) {
			deviceID := ids.DeviceID(sourceID)
			files := m.evaluateDeviceSource(deviceID, scope.Filters)
			for fid := range files {
				resultSet[[2]uint64{uint64(deviceID), uint64(fid)}] = struct{}{}
			}
		} else {
			subScopeID := ids.ScopeID(sourceID)
			subResults, err := m.List(ctx, subScopeID)
			if err != nil {
				continue
			}
			for _, pair := range subResults {
				resultSet[pair] = struct{}{}
			}
		}
	}

	scope.Cache = resultSet
	scope.CacheValid = true

	result := make([][2]uint64, 0, len(resultSet))
	for pair := range resultSet {
		result = append(result, pair)
	}

	return result, nil
}

func (m *Manager) evaluateDeviceSource(deviceID ids.DeviceID, filters map[string]struct{}) index.FileSet {
	if len(filters) == 0 {
		return index.NewFileSet()
	}

	sets := []index.FileSet{}
	for tagName := range filters {
		tid, found := m.tagListProvider.FindTagID(tagName)
		if !found {
			return index.NewFileSet()
		}

		files := m.tagListProvider.Files(tid)
		fileSet := index.NewFileSet()
		for _, fid := range files {
			fileSet.Add(fid)
		}
		sets = append(sets, fileSet)
	}

	return index.Intersect(sets)
}

func (m *Manager) invalidateScope(scope *Scope) {
	scope.CacheValid = false
	scope.Cache = make(map[[2]uint64]struct{})

	for subScopeID := range scope.SubScopes {
		if subScope, exists := m.scopes[subScopeID]; exists {
			m.invalidateScope(subScope)
		}
	}
}

func (m *Manager) hasCycle(from, to ids.ScopeID) bool {
	visited := make(map[ids.ScopeID]struct{})
	return m.hasCycleHelper(to, from, visited)
}

func (m *Manager) hasCycleHelper(current, target ids.ScopeID, visited map[ids.ScopeID]struct{}) bool {
	if current == target {
		return true
	}

	if _, seen := visited[current]; seen {
		return false
	}
	visited[current] = struct{}{}

	scope, exists := m.scopes[current]
	if !exists {
		return false
	}

	for sourceID := range scope.Sources {
		if sourceID >= uint64(1<<32) {
			subScopeID := ids.ScopeID(sourceID)
			if m.hasCycleHelper(subScopeID, target, visited) {
				return true
			}
		}
	}

	return false
}

func (m *Manager) UpdateCachesForTag(ctx context.Context, tagName string, deviceID ids.DeviceID, fileID ids.FileID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	scopeSet, exists := m.tagScopeRef[tagName]
	if !exists {
		return
	}

	for scopeID := range scopeSet {
		if scope, exists := m.scopes[scopeID]; exists {
			m.invalidateScope(scope)
		}
	}
}
