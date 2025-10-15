package main

import "context"

// Dep defines lifecycle for a dependency component.
type Dep interface {
	// Init initializes the dependency using the provided context and a
	// reference to the container `Dependencies`. The container is passed so
	// implementations can resolve other dependencies during initialization.
	// The context may be canceled by the caller to abort initialization.
	Init(ctx context.Context, deps *Dependencies) error

	// GetName returns the unique name/key of this dependency.
	GetName() string

	// GetRefs returns the list of required dependency names (keys).
	GetRefs() []string

	// Get returns the prepared instance. May return nil if not initialized.
	Get() any

	// Close stops the dependency and releases resources. The provided
	// context may be canceled by the caller to force shutdown.
	Close(ctx context.Context) error
}
