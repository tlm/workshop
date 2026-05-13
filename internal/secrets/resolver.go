// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3.

// Package secrets provides the routing engine that resolves a secret
// plug request into a concrete secret value via the provider registry.
package secrets

import (
	"context"
	"fmt"

	"github.com/canonical/workshop/internal/interfaces"
)

// ProviderLookup is the signature for looking up a provider by name.
type ProviderLookup func(name string) (Provider, bool)

// ErrUnroutedSecret is returned when a secret plug is not connected
// to a slot.
var ErrUnroutedSecret = fmt.Errorf(
	"secret plug is not connected to a slot",
)

// SecretResolverFunc is a resolver that has already been bound to a
// specific SDK. The caller only needs to supply the plug name.
type SecretResolverFunc func(ctx context.Context, plugName string) (string, error)

// Resolver resolves secret plugs into secret values by traversing the
// connection graph maintained by the interface repository.
type Resolver struct {
	repo   *interfaces.Repository
	lookup ProviderLookup
}

// NewResolver creates a Resolver backed by the given interface
// repository and provider lookup.
func NewResolver(
	repo *interfaces.Repository,
	lookup ProviderLookup,
) *Resolver {
	return &Resolver{repo: repo, lookup: lookup}
}

// Bind returns a SecretResolverFunc pre-bound to the given SDK
// coordinates. The caller only needs to supply the plug name.
func (r *Resolver) Bind(
	projectID, workshop, sdkName string,
) SecretResolverFunc {
	return func(ctx context.Context, plugName string) (string, error) {
		return r.ResolvePlug(
			ctx, projectID, workshop, sdkName, plugName,
		)
	}
}

// ResolvePlug looks up the named secret plug for the given SDK,
// traverses its connection to a slot, fetches the corresponding
// provider, and returns the resolved secret value.
//
// If the plug does not exist, is not connected, or its provider is
// not registered, a wrapped error is returned.
func (r *Resolver) ResolvePlug(
	ctx context.Context,
	projectID, workshop, sdkName, plugName string,
) (string, error) {
	plug := r.repo.Plug(projectID, workshop, sdkName, plugName)
	if plug == nil {
		return "", fmt.Errorf(
			"secret plug %q not found for %s/%s:%s",
			plugName, projectID, workshop, sdkName,
		)
	}

	conns, err := r.repo.Connected(
		projectID, workshop, sdkName, plugName,
	)
	if err != nil {
		return "", fmt.Errorf(
			"cannot determine connections for secret plug %q: %w",
			plugName, err,
		)
	}
	if len(conns) == 0 {
		return "", fmt.Errorf(
			"secret plug %q is not connected to a slot: %w",
			plugName, ErrUnroutedSecret,
		)
	}
	if len(conns) > 1 {
		return "", fmt.Errorf(
			"secret plug %q is connected to multiple slots",
			plugName,
		)
	}

	connRef := conns[0]
	conn, err := r.repo.Connection(connRef)
	if err != nil {
		return "", fmt.Errorf(
			"cannot retrieve connection for secret plug %q: %w",
			plugName, err,
		)
	}

	var providerName string
	if err := conn.Slot.Attr("provider", &providerName); err != nil {
		return "", fmt.Errorf(
			"secret slot for plug %q has no provider: %w",
			plugName, err,
		)
	}

	var source string
	if err := conn.Slot.Attr("source", &source); err != nil {
		return "", fmt.Errorf(
			"secret slot for plug %q has no source: %w",
			plugName, err,
		)
	}

	prov, ok := r.lookup(providerName)
	if !ok {
		return "", fmt.Errorf(
			"secret provider %q for plug %q is not registered",
			providerName, plugName,
		)
	}

	value, err := prov.Resolve(ctx, source)
	if err != nil {
		return "", fmt.Errorf(
			"cannot resolve secret for plug %q via provider %q: %w",
			plugName, providerName, err,
		)
	}

	return value, nil
}
