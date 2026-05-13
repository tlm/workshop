// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3.

// Package secrets defines the interface for secret providers and a registry
// to manage them.
package secrets

import "context"

// Provider is the interface that secret backends must implement.
type Provider interface {
	// Name returns the unique identifier for this provider (e.g. "host-env").
	Name() string

	// Resolve retrieves the value of the secret identified by source.
	Resolve(ctx context.Context, source string) (string, error)
}
