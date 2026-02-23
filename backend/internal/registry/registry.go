package registry

import (
	"fmt"
	"sync"
)

// --- AdapterRegistry ---

// AdapterRegistry is a thread-safe registry for MarketAdapter implementations
type AdapterRegistry struct {
	mu       sync.RWMutex
	adapters map[string]MarketAdapter
}

var adapterOnce sync.Once
var adapterInstance *AdapterRegistry

// Adapters returns the global AdapterRegistry singleton
func Adapters() *AdapterRegistry {
	adapterOnce.Do(func() {
		adapterInstance = &AdapterRegistry{
			adapters: make(map[string]MarketAdapter),
		}
	})
	return adapterInstance
}

// Register adds a MarketAdapter to the registry
func (r *AdapterRegistry) Register(adapter MarketAdapter) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := adapter.Name()
	if _, exists := r.adapters[name]; exists {
		return fmt.Errorf("adapter %q is already registered", name)
	}
	r.adapters[name] = adapter
	return nil
}

// Get retrieves a MarketAdapter by name
func (r *AdapterRegistry) Get(name string) (MarketAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	adapter, ok := r.adapters[name]
	if !ok {
		return nil, fmt.Errorf("adapter %q not found", name)
	}
	return adapter, nil
}

// All returns all registered adapters
func (r *AdapterRegistry) All() []MarketAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]MarketAdapter, 0, len(r.adapters))
	for _, a := range r.adapters {
		result = append(result, a)
	}
	return result
}

// Names returns the names of all registered adapters
func (r *AdapterRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	return names
}

// --- StrategyRegistry ---

// StrategyFactory is a function that creates a new Strategy instance
type StrategyFactory func() Strategy

// StrategyRegistry is a thread-safe registry for Strategy factories
type StrategyRegistry struct {
	mu        sync.RWMutex
	factories map[string]StrategyFactory
}

var strategyOnce sync.Once
var strategyInstance *StrategyRegistry

// Strategies returns the global StrategyRegistry singleton
func Strategies() *StrategyRegistry {
	strategyOnce.Do(func() {
		strategyInstance = &StrategyRegistry{
			factories: make(map[string]StrategyFactory),
		}
	})
	return strategyInstance
}

// Register adds a StrategyFactory to the registry
func (r *StrategyRegistry) Register(name string, factory StrategyFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("strategy %q is already registered", name)
	}
	r.factories[name] = factory
	return nil
}

// Create instantiates a new Strategy by name
func (r *StrategyRegistry) Create(name string) (Strategy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, ok := r.factories[name]
	if !ok {
		return nil, fmt.Errorf("strategy %q not found", name)
	}
	return factory(), nil
}

// Names returns the names of all registered strategies
func (r *StrategyRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}

// Exists returns true if the strategy is registered
func (r *StrategyRegistry) Exists(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.factories[name]
	return ok
}
