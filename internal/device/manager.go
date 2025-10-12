package device

import (
	"sync"

	"github.com/sekai02/redcloud-files/internal/ids"
)

type Manager struct {
	mu           sync.RWMutex
	devices      map[ids.DeviceID]string
	idGen        *ids.Generator
	hwIDToDevice map[string]ids.DeviceID
}

func NewManager(idGen *ids.Generator) *Manager {
	return &Manager{
		devices:      make(map[ids.DeviceID]string),
		idGen:        idGen,
		hwIDToDevice: make(map[string]ids.DeviceID),
	}
}

func (m *Manager) RegisterDevice(hwID string) ids.DeviceID {
	m.mu.Lock()
	defer m.mu.Unlock()

	if devID, exists := m.hwIDToDevice[hwID]; exists {
		return devID
	}

	devID := m.idGen.NextDevice()
	m.devices[devID] = hwID
	m.hwIDToDevice[hwID] = devID
	return devID
}

func (m *Manager) List() []ids.DeviceID {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ids.DeviceID, 0, len(m.devices))
	for devID := range m.devices {
		result = append(result, devID)
	}
	return result
}

func (m *Manager) Exists(devID ids.DeviceID) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.devices[devID]
	return exists
}

func (m *Manager) SnapshotDevices() map[ids.DeviceID]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[ids.DeviceID]string)
	for k, v := range m.devices {
		result[k] = v
	}
	return result
}

func (m *Manager) SnapshotHwIDMap() map[string]ids.DeviceID {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]ids.DeviceID)
	for k, v := range m.hwIDToDevice {
		result[k] = v
	}
	return result
}

func (m *Manager) RestoreDevices(devices map[ids.DeviceID]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.devices = devices
}

func (m *Manager) RestoreHwIDMap(hwIDMap map[string]ids.DeviceID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.hwIDToDevice = hwIDMap
}
