package device

import (
	"fmt"
	"sync"
)

var (
	registry     []DeviceProfile
	registryOnce sync.Once
	registryMu   sync.RWMutex
)

// initRegistry initializes the registry with built-in profiles.
func initRegistry() {
	registryOnce.Do(func() {
		// Register built-in profiles
		Register(&QTKeJiProfile{})
		// Add more built-in profiles here as they are implemented
	})
}

// Register adds a device profile to the registry.
// This is typically called during package initialization.
func Register(profile DeviceProfile) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = append(registry, profile)
}

// All returns all registered device profiles.
func All() []DeviceProfile {
	initRegistry()
	registryMu.RLock()
	defer registryMu.RUnlock()

	result := make([]DeviceProfile, len(registry))
	copy(result, registry)
	return result
}

// FindByVIDPID finds a profile that matches the given vendor and product ID.
// Returns nil if no matching profile is found.
func FindByVIDPID(vendorID, productID uint16) DeviceProfile {
	initRegistry()
	registryMu.RLock()
	defer registryMu.RUnlock()

	for _, p := range registry {
		if p.Matches(vendorID, productID) {
			return p
		}
	}
	return nil
}

// FindByID finds a profile by its unique ID.
// Returns nil if no matching profile is found.
func FindByID(id string) DeviceProfile {
	initRegistry()
	registryMu.RLock()
	defer registryMu.RUnlock()

	for _, p := range registry {
		if p.ID() == id {
			return p
		}
	}
	return nil
}

// MustFindByVIDPID finds a profile or returns a generic fallback.
// This never returns nil - if no specific profile matches, a GenericProfile is returned.
func MustFindByVIDPID(vendorID, productID uint16) DeviceProfile {
	if p := FindByVIDPID(vendorID, productID); p != nil {
		return p
	}
	return NewGenericProfile(vendorID, productID)
}

// ListProfiles returns information about all registered profiles.
func ListProfiles() []ProfileInfo {
	profiles := All()
	result := make([]ProfileInfo, len(profiles))
	for i, p := range profiles {
		result[i] = GetInfo(p)
	}
	return result
}

// IsKnownDevice returns true if the VID/PID matches a known device profile.
func IsKnownDevice(vendorID, productID uint16) bool {
	return FindByVIDPID(vendorID, productID) != nil
}

// KnownVIDPIDs returns all known VID/PID pairs from registered profiles.
func KnownVIDPIDs() [][2]uint16 {
	initRegistry()
	registryMu.RLock()
	defer registryMu.RUnlock()

	var pairs [][2]uint16
	for _, p := range registry {
		vids := p.VendorIDs()
		pids := p.ProductIDs()
		for _, vid := range vids {
			for _, pid := range pids {
				pairs = append(pairs, [2]uint16{vid, pid})
			}
		}
	}
	return pairs
}

// FormatVIDPID formats a VID/PID pair as a string.
func FormatVIDPID(vendorID, productID uint16) string {
	return fmt.Sprintf("%04x:%04x", vendorID, productID)
}
