// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3.

package builtin

import (
	"fmt"

	"github.com/canonical/workshop/internal/secrets"
)

var allProviders map[string]secrets.Provider

// registerProvider adds a provider to the set of known providers.
// It panics if a provider with the same name is already registered.
func registerProvider(p secrets.Provider) {
	if allProviders[p.Name()] != nil {
		panic(fmt.Errorf("cannot register duplicate secret provider %q", p.Name()))
	}
	if allProviders == nil {
		allProviders = make(map[string]secrets.Provider)
	}
	allProviders[p.Name()] = p
}

// GetProvider returns the provider registered under name. The boolean is
// false when no provider with that name exists.
func GetProvider(name string) (secrets.Provider, bool) {
	p, ok := allProviders[name]
	return p, ok
}

// MockProvider replaces (or adds) a provider for testing and returns a
// restore function.
func MockProvider(p secrets.Provider) func() {
	name := p.Name()
	if allProviders == nil {
		allProviders = make(map[string]secrets.Provider)
	}
	allProviders[name] = p
	return func() {
		delete(allProviders, name)
	}
}
