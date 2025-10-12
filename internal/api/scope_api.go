package api

import (
	"context"

	"github.com/sekai02/redcloud-files/internal/ids"
)

func (s *Service) MkScope(ctx context.Context) (uint64, error) {
	sid, err := s.scopeMgr.MkScope(ctx)
	if err != nil {
		return 0, err
	}
	return uint64(sid), nil
}

func (s *Service) ScopeAddSource(ctx context.Context, scopeID, sourceID uint64) error {
	sid := ids.ScopeID(scopeID)
	return s.scopeMgr.AddSource(ctx, sid, sourceID)
}

func (s *Service) ScopeRmSource(ctx context.Context, scopeID, sourceID uint64) error {
	sid := ids.ScopeID(scopeID)
	return s.scopeMgr.RmSource(ctx, sid, sourceID)
}

func (s *Service) ScopeAddFilter(ctx context.Context, scopeID uint64, tags ...string) error {
	sid := ids.ScopeID(scopeID)
	return s.scopeMgr.AddFilter(ctx, sid, tags...)
}

func (s *Service) ScopeRmFilter(ctx context.Context, scopeID uint64, tags ...string) error {
	sid := ids.ScopeID(scopeID)
	return s.scopeMgr.RmFilter(ctx, sid, tags...)
}

func (s *Service) List(ctx context.Context, scopeID uint64) ([][2]uint64, error) {
	sid := ids.ScopeID(scopeID)
	return s.scopeMgr.List(ctx, sid)
}
