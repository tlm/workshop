# Task Spec: Implement Secret Provider Interface and Registry

**Task ID:** SEC-004
**Role:** Backend Go Developer
**Status:** Ready for Dev

## Objective
Define a generic Go interface for Secret Providers and implement a registry to manage them. This allows the `system` SDK to dynamically route secret requests to different backends (like `host-env` or `mock`) without hardcoding the retrieval logic.

## Context
When a `secret` plug is bound to a `secret` slot, Workshop needs to resolve the secret at runtime. The slot definition contains a `provider` (e.g., `host-env`) and a `source` (e.g., `AWS_ACCESS_KEY_ID`). Workshop will use the Provider Registry to look up the correct provider by name and call its `Resolve` method.

## Scope of Work

1. **Core Interfaces (`internal/secrets/provider.go`):**
   - Create a new package `internal/secrets`.
   - Define the `Provider` interface with `Name() string` and `Resolve(ctx context.Context, source string) (string, error)`.
   - Define standard errors: `ErrSecretNotFound` and `ErrSecretDenied`. Any other returned error should be treated as a generic provider failure.

2. **Provider Registry (`internal/secrets/registry.go`):**
   - Implement a thread-safe `Registry` struct to hold registered providers.
   - Add `Register(p Provider)` and `Get(name string) (Provider, bool)` methods.
   - Initialize a global default registry that Workshop can use at startup.

3. **Implement the `mock` Provider (`internal/secrets/providers/mock.go`):**
   - Create a simple `mock` provider that implements the `Provider` interface.
   - `Name()` should return `"mock"`.
   - `Resolve()` should return a predictable string based on the source (e.g., `"mock-value-for-" + source`).
   - Register this provider with the global registry in an `init()` function or during daemon startup.

## Acceptance Criteria
- [ ] The `internal/secrets` package is created with the `Provider` and `Registry` interfaces.
- [ ] Standard error types (`ErrSecretNotFound`, `ErrSecretDenied`) are defined.
- [ ] A `mock` provider is fully implemented and registered.
- [ ] Unit tests are written for the registry and the `mock` provider.

## Out of Scope
- Implementing the `host-env` provider (this will be a separate task).
- Wiring this registry into the actual Workshop interface connection/launch logic.