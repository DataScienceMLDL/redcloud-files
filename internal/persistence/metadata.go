package persistence

import (
	"encoding/json"
	"fmt"

	"github.com/sekai02/redcloud-files/internal/device"
	"github.com/sekai02/redcloud-files/internal/ids"
	"github.com/sekai02/redcloud-files/internal/inode"
	"github.com/sekai02/redcloud-files/internal/tag/taglist"
	"github.com/sekai02/redcloud-files/internal/tag/tagtree"
)

type SystemMetadata struct {
	IDGen   ids.GeneratorSnapshot
	Devices DeviceSnapshot
	Inodes  map[ids.DeviceID]map[ids.FileID]inode.InodeData
	Tags    taglist.TagListSnapshot
}

type DeviceSnapshot struct {
	Devices   map[ids.DeviceID]string
	HwIDToDevice map[string]ids.DeviceID
}

func SaveMetadata(
	idGen *ids.Generator,
	deviceMgr *device.Manager,
	inodeMgr *inode.Manager,
	tagList *taglist.List,
) ([]byte, error) {
	metadata := SystemMetadata{
		IDGen: idGen.Snapshot(),
		Devices: DeviceSnapshot{
			Devices:      deviceMgr.SnapshotDevices(),
			HwIDToDevice: deviceMgr.SnapshotHwIDMap(),
		},
		Inodes: inodeMgr.SnapshotInodes(),
		Tags:   tagList.Snapshot(),
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	return data, nil
}

func LoadMetadata(
	data []byte,
	idGen *ids.Generator,
	deviceMgr *device.Manager,
	inodeMgr *inode.Manager,
	tagList *taglist.List,
	tagTree *tagtree.Tree,
) error {
	var metadata SystemMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("unmarshal metadata: %w", err)
	}

	idGen.Restore(metadata.IDGen)
	deviceMgr.RestoreDevices(metadata.Devices.Devices)
	deviceMgr.RestoreHwIDMap(metadata.Devices.HwIDToDevice)
	inodeMgr.RestoreInodes(metadata.Inodes)
	tagList.Restore(metadata.Tags)

	for tid := range metadata.Tags.Tags {
		tagName := tagList.TagName(tid)
		if tagName != "" {
			tagTree.Insert(tagName, tid)
		}
	}

	return nil
}
