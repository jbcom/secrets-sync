package driver

import (
	"fmt"
	"sort"
	"sync"
)

// BackendSpec is the provider-agnostic description of a backend instance the
// pipeline wants to construct. Concrete factories interpret Options according
// to their driver. Path is the scoping path/prefix; Options carries
// driver-specific settings (region, mount, namespace, endpoint, auth, ...).
type BackendSpec struct {
	Driver  DriverName
	Path    string
	Options map[string]any
}

// SourceFactory builds a SourceBackend from a spec.
type SourceFactory func(spec BackendSpec) (SourceBackend, error)

// TargetFactory builds a TargetBackend from a spec.
type TargetFactory func(spec BackendSpec) (TargetBackend, error)

// Registry maps driver names to backend factories. It is safe for concurrent
// use. Providers register themselves (typically from an init function in their
// own package) so pkg/driver never imports the concrete client packages,
// avoiding an import cycle.
type Registry struct {
	mu      sync.RWMutex
	sources map[DriverName]SourceFactory
	targets map[DriverName]TargetFactory
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		sources: map[DriverName]SourceFactory{},
		targets: map[DriverName]TargetFactory{},
	}
}

// Default is the process-wide registry providers register into.
var Default = NewRegistry()

// RegisterSource registers a source factory for a driver. It panics on a
// duplicate registration, which always indicates a programming error.
func (r *Registry) RegisterSource(name DriverName, f SourceFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.sources[name]; exists {
		panic(fmt.Sprintf("driver: source already registered for %q", name))
	}
	r.sources[name] = f
}

// RegisterTarget registers a target factory for a driver. A TargetBackend is
// also a SourceBackend, so registering a target additionally exposes it as a
// source unless a dedicated source factory is registered.
func (r *Registry) RegisterTarget(name DriverName, f TargetFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.targets[name]; exists {
		panic(fmt.Sprintf("driver: target already registered for %q", name))
	}
	r.targets[name] = f
	if _, exists := r.sources[name]; !exists {
		r.sources[name] = func(spec BackendSpec) (SourceBackend, error) {
			tb, err := f(spec)
			if err != nil {
				return nil, err
			}
			return tb, nil
		}
	}
}

// NewSource constructs a source backend for the given spec.
func (r *Registry) NewSource(spec BackendSpec) (SourceBackend, error) {
	r.mu.RLock()
	f, ok := r.sources[spec.Driver]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("driver: no source backend registered for %q", spec.Driver)
	}
	return f(spec)
}

// NewTarget constructs a target backend for the given spec.
func (r *Registry) NewTarget(spec BackendSpec) (TargetBackend, error) {
	r.mu.RLock()
	f, ok := r.targets[spec.Driver]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("driver: no target backend registered for %q", spec.Driver)
	}
	return f(spec)
}

// SupportsSource reports whether a source factory is registered for the driver.
func (r *Registry) SupportsSource(name DriverName) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.sources[name]
	return ok
}

// SupportsTarget reports whether a target factory is registered for the driver.
func (r *Registry) SupportsTarget(name DriverName) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.targets[name]
	return ok
}

// Drivers returns the sorted set of driver names with at least one registered
// factory (source or target).
func (r *Registry) Drivers() []DriverName {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := map[DriverName]struct{}{}
	for n := range r.sources {
		seen[n] = struct{}{}
	}
	for n := range r.targets {
		seen[n] = struct{}{}
	}
	out := make([]DriverName, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
