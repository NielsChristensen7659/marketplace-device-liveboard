package marketplace

import (
	"errors"
	"sync"
)

var ErrNotOwner = errors.New("account does not own device")

type Device struct {
	ID      string `json:"id"`
	OwnerID string `json:"owner_id"`
	Status  string `json:"status"`
}

type Registry struct {
	mu      sync.RWMutex
	devices map[string]Device
}

func NewRegistry(devices []Device) *Registry {
	indexed := make(map[string]Device, len(devices))
	for _, device := range devices {
		indexed[device.ID] = device
	}
	return &Registry{devices: indexed}
}

func (r *Registry) Handoff(deviceID, sellerID, buyerID string) (Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	device, ok := r.devices[deviceID]
	if !ok || device.OwnerID != sellerID {
		return Device{}, ErrNotOwner
	}
	device.OwnerID = buyerID
	r.devices[deviceID] = device
	return device, nil
}

func (r *Registry) UpdateStatus(deviceID, accountID, status string) (Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	device, ok := r.devices[deviceID]
	if !ok || device.OwnerID != accountID {
		return Device{}, ErrNotOwner
	}
	device.Status = status
	r.devices[deviceID] = device
	return device, nil
}
