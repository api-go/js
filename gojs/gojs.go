package gojs

import "sync"

var (
	modules = make(map[string]map[string]any)
	mu      sync.RWMutex
)

// Register registers a Go module with its exported functions and properties.
// It is thread-safe and designed to be called during init() functions.
// If the same module name is registered multiple times, the exports are merged.
func Register(name string, exports map[string]any) {
	mu.Lock()
	defer mu.Unlock()

	if modules[name] == nil {
		modules[name] = make(map[string]any, len(exports))
	}

	for k, v := range exports {
		modules[name][k] = v
	}
}

// GetModules returns all registered modules.
// It is thread-safe and safe to iterate during runtime.
func GetModules() map[string]map[string]any {
	mu.RLock()
	defer mu.RUnlock()

	res := make(map[string]map[string]any, len(modules))
	for name, exports := range modules {
		res[name] = exports // exports map is treated as read-only after initialization
	}
	return res
}
