package simple_di

import "context"

// DepName is a simple alias for dependency name keys.
type DepName string

// Dependency defines lifecycle for a dependency component.
type Dependency interface {
	// Init initializes the dependency using the provided context and a
	// reference to the container `Dependencies`. The container is passed so
	// implementations can resolve other dependencies during initialization.
	// The context may be canceled by the caller to abort initialization.
	Init(ctx context.Context, deps *Container) error

	// GetName returns the unique name/key of this dependency.
	GetName() DepName

	// GetRefs returns the list of required dependency names (keys).
	GetRefs() []DepName

	// Get returns the prepared instance. May return nil if not initialized.
	Get() any

	// Close stops the dependency and releases resources. The provided
	// context may be canceled by the caller to force shutdown.
	Close(ctx context.Context) error
}
