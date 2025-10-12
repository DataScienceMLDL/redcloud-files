package api

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/sekai02/redcloud-files/internal/device"
	"github.com/sekai02/redcloud-files/internal/ids"
	"github.com/sekai02/redcloud-files/internal/inode"
	"github.com/sekai02/redcloud-files/internal/persistence"
	"github.com/sekai02/redcloud-files/internal/scope"
	"github.com/sekai02/redcloud-files/internal/storage"
	"github.com/sekai02/redcloud-files/internal/tag/taglist"
	"github.com/sekai02/redcloud-files/internal/tag/tagtree"
)

type Service struct {
	deviceMgr  *device.Manager
	inodeMgr   *inode.Manager
	tagList    *taglist.List
	tagTree    *tagtree.Tree
	scopeMgr   *scope.Manager
	idGen      *ids.Generator
	store      storage.Store
}

func NewService(
	deviceMgr *device.Manager,
	inodeMgr *inode.Manager,
	tagList *taglist.List,
	tagTree *tagtree.Tree,
	scopeMgr *scope.Manager,
	idGen *ids.Generator,
	store storage.Store,
) *Service {
	return &Service{
		deviceMgr:  deviceMgr,
		inodeMgr:   inodeMgr,
		tagList:    tagList,
		tagTree:    tagTree,
		scopeMgr:   scopeMgr,
		idGen:      idGen,
		store:      store,
	}
}

func (s *Service) persistMetadata() {
	data, err := persistence.SaveMetadata(s.idGen, s.deviceMgr, s.inodeMgr, s.tagList)
	if err != nil {
		slog.Error("Failed to serialize metadata", "error", err)
		return
	}

	err = s.store.SaveMetadata("system", data)
	if err != nil {
		slog.Error("Failed to save metadata to storage", "error", err)
	}
}

func (s *Service) Create(ctx context.Context, deviceID uint64) (uint64, error) {
	devID := ids.DeviceID(deviceID)

	if !s.deviceMgr.Exists(devID) {
		return 0, fmt.Errorf("device %d not found", deviceID)
	}

	fid, err := s.inodeMgr.Create(ctx, devID)
	if err != nil {
		return 0, fmt.Errorf("create file: %w", err)
	}

	s.persistMetadata()
	return uint64(fid), nil
}

func (s *Service) Copy(ctx context.Context, deviceID, fileID, destinationDeviceID uint64) (uint64, error) {
	srcDevID := ids.DeviceID(deviceID)
	srcFileID := ids.FileID(fileID)
	dstDevID := ids.DeviceID(destinationDeviceID)

	if !s.deviceMgr.Exists(srcDevID) {
		return 0, fmt.Errorf("source device %d not found", deviceID)
	}
	if !s.deviceMgr.Exists(dstDevID) {
		return 0, fmt.Errorf("destination device %d not found", destinationDeviceID)
	}

	if !s.inodeMgr.Exists(ctx, srcDevID, srcFileID) {
		return 0, fmt.Errorf("file %d not found on device %d", fileID, deviceID)
	}

	data, err := s.inodeMgr.Read(ctx, srcDevID, srcFileID, 0, 1<<30)
	if err != nil {
		return 0, fmt.Errorf("read source file: %w", err)
	}

	dstFileID, err := s.inodeMgr.Create(ctx, dstDevID)
	if err != nil {
		return 0, fmt.Errorf("create destination file: %w", err)
	}

	if len(data) > 0 {
		_, err = s.inodeMgr.Write(ctx, dstDevID, dstFileID, 0, data)
		if err != nil {
			s.inodeMgr.Delete(ctx, dstDevID, dstFileID)
			return 0, fmt.Errorf("write destination file: %w", err)
		}
	}

	tagIDs, err := s.inodeMgr.TagIDs(ctx, srcDevID, srcFileID)
	if err != nil {
		return uint64(dstFileID), nil
	}

	for _, tid := range tagIDs {
		tagName := s.tagList.TagName(tid)
		if tagName != "" {
			s.TagAdd(ctx, destinationDeviceID, uint64(dstFileID), tagName)
		}
	}

	s.persistMetadata()
	return uint64(dstFileID), nil
}

func (s *Service) Delete(ctx context.Context, deviceID, fileID uint64) error {
	devID := ids.DeviceID(deviceID)
	fid := ids.FileID(fileID)

	if !s.deviceMgr.Exists(devID) {
		return fmt.Errorf("device %d not found", deviceID)
	}

	tagIDs, err := s.inodeMgr.TagIDs(ctx, devID, fid)
	if err != nil {
		return fmt.Errorf("get tags: %w", err)
	}

	for _, tid := range tagIDs {
		s.tagList.RemoveFID(tid, fid)
		tagName := s.tagList.TagName(tid)
		if tagName != "" {
			s.scopeMgr.UpdateCachesForTag(ctx, tagName, devID, fid)
		}
	}

	err = s.inodeMgr.Delete(ctx, devID, fid)
	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}

	s.persistMetadata()
	return nil
}

func (s *Service) Read(ctx context.Context, deviceID, fileID uint64, offset, length int64) ([]byte, error) {
	devID := ids.DeviceID(deviceID)
	fid := ids.FileID(fileID)

	if !s.deviceMgr.Exists(devID) {
		return nil, fmt.Errorf("device %d not found", deviceID)
	}

	data, err := s.inodeMgr.Read(ctx, devID, fid, offset, length)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return data, nil
}

func (s *Service) Write(ctx context.Context, deviceID, fileID uint64, offset int64, data []byte) (int, error) {
	devID := ids.DeviceID(deviceID)
	fid := ids.FileID(fileID)

	if !s.deviceMgr.Exists(devID) {
		return 0, fmt.Errorf("device %d not found", deviceID)
	}

	n, err := s.inodeMgr.Write(ctx, devID, fid, offset, data)
	if err != nil {
		return 0, fmt.Errorf("write file: %w", err)
	}

	s.persistMetadata()
	return n, nil
}

func (s *Service) TagAdd(ctx context.Context, deviceID, fileID uint64, tagName string) error {
	devID := ids.DeviceID(deviceID)
	fid := ids.FileID(fileID)

	if !s.deviceMgr.Exists(devID) {
		return fmt.Errorf("device %d not found", deviceID)
	}

	if !s.inodeMgr.Exists(ctx, devID, fid) {
		return fmt.Errorf("file %d not found on device %d", fileID, deviceID)
	}

	tid, found := s.tagTree.Lookup(tagName)
	if !found {
		tid, _ = s.tagList.AllocTagID(tagName)
		s.tagTree.Insert(tagName, tid)
	}

	err := s.tagList.AddFID(tid, fid)
	if err != nil {
		return fmt.Errorf("add file to tag list: %w", err)
	}

	err = s.inodeMgr.AddTagID(ctx, devID, fid, tid)
	if err != nil {
		return fmt.Errorf("add tag to inode: %w", err)
	}

	s.scopeMgr.UpdateCachesForTag(ctx, tagName, devID, fid)

	s.persistMetadata()
	return nil
}

func (s *Service) TagRemove(ctx context.Context, deviceID, fileID uint64, tagName string) error {
	devID := ids.DeviceID(deviceID)
	fid := ids.FileID(fileID)

	if !s.deviceMgr.Exists(devID) {
		return fmt.Errorf("device %d not found", deviceID)
	}

	if !s.inodeMgr.Exists(ctx, devID, fid) {
		return fmt.Errorf("file %d not found on device %d", fileID, deviceID)
	}

	tid, found := s.tagTree.Lookup(tagName)
	if !found {
		return nil
	}

	err := s.tagList.RemoveFID(tid, fid)
	if err != nil && err.Error() != fmt.Sprintf("file %d not found in tag %d", fid, tid) {
		return fmt.Errorf("remove file from tag list: %w", err)
	}

	err = s.inodeMgr.RemoveTagID(ctx, devID, fid, tid)
	if err != nil {
		return fmt.Errorf("remove tag from inode: %w", err)
	}

	s.scopeMgr.UpdateCachesForTag(ctx, tagName, devID, fid)

	s.persistMetadata()
	return nil
}

func (s *Service) TagList(ctx context.Context, deviceID, fileID uint64) ([]string, error) {
	devID := ids.DeviceID(deviceID)
	fid := ids.FileID(fileID)

	if !s.deviceMgr.Exists(devID) {
		return nil, fmt.Errorf("device %d not found", deviceID)
	}

	if !s.inodeMgr.Exists(ctx, devID, fid) {
		return nil, fmt.Errorf("file %d not found on device %d", fileID, deviceID)
	}

	tagIDs, err := s.inodeMgr.TagIDs(ctx, devID, fid)
	if err != nil {
		return nil, fmt.Errorf("get tag IDs: %w", err)
	}

	result := make([]string, 0, len(tagIDs))
	for _, tid := range tagIDs {
		tagName := s.tagList.TagName(tid)
		if tagName != "" {
			result = append(result, tagName)
		}
	}

	return result, nil
}

func (s *Service) DeviceList(ctx context.Context) ([]uint64, error) {
	devices := s.deviceMgr.List()
	result := make([]uint64, len(devices))
	for i, dev := range devices {
		result[i] = uint64(dev)
	}
	return result, nil
}

func (s *Service) ImportFile(ctx context.Context, deviceID uint64, osPath string, tags []string) (uint64, string, error) {
	devID := ids.DeviceID(deviceID)

	if !s.deviceMgr.Exists(devID) {
		return 0, "", fmt.Errorf("device %d not found", deviceID)
	}

	data, err := os.ReadFile(osPath)
	if err != nil {
		return 0, "", fmt.Errorf("read OS file: %w", err)
	}

	fid, err := s.inodeMgr.Create(ctx, devID)
	if err != nil {
		return 0, "", fmt.Errorf("create file: %w", err)
	}

	if len(data) > 0 {
		_, err = s.inodeMgr.Write(ctx, devID, fid, 0, data)
		if err != nil {
			s.inodeMgr.Delete(ctx, devID, fid)
			return 0, "", fmt.Errorf("write file data: %w", err)
		}
	}

	for _, tag := range tags {
		if tag != "" {
			err = s.TagAdd(ctx, deviceID, uint64(fid), tag)
			if err != nil {
				return uint64(fid), filepath.Base(osPath), fmt.Errorf("add tag %s: %w", tag, err)
			}
		}
	}

	s.persistMetadata()
	return uint64(fid), filepath.Base(osPath), nil
}

func (s *Service) ExportFile(ctx context.Context, deviceID, fileID uint64, osPath string) error {
	devID := ids.DeviceID(deviceID)
	fid := ids.FileID(fileID)

	if !s.deviceMgr.Exists(devID) {
		return fmt.Errorf("device %d not found", deviceID)
	}

	if !s.inodeMgr.Exists(ctx, devID, fid) {
		return fmt.Errorf("file %d not found on device %d", fileID, deviceID)
	}

	data, err := s.inodeMgr.Read(ctx, devID, fid, 0, 1<<30)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	err = os.WriteFile(osPath, data, 0644)
	if err != nil {
		return fmt.Errorf("write OS file: %w", err)
	}

	return nil
}
