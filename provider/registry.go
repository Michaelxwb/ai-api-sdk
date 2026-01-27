package provider

import "sync"

var (
	regMu sync.RWMutex
	reg   = map[string]ProviderSpec{}
)

// Register registers a provider spec.
func Register(name string, spec ProviderSpec) {
	if spec == nil || name == "" {
		return
	}
	regMu.Lock()
	reg[name] = spec
	regMu.Unlock()
}

// Get returns a provider spec by name.
func Get(name string) (ProviderSpec, bool) {
	regMu.RLock()
	spec, ok := reg[name]
	regMu.RUnlock()
	return spec, ok
}

// List returns all provider specs.
func List() map[string]ProviderSpec {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make(map[string]ProviderSpec, len(reg))
	for k, v := range reg {
		out[k] = v
	}
	return out
}
