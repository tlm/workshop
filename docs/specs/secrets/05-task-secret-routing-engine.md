# Task Spec: Implement Secret Routing & Connection Engine

**Task ID:** SEC-005
**Role:** Backend Go Developer
**Status:** In Progress — disconnect cache purge deferred until a caching provider exists

## Objective
Implement the `secret` interface backend in Workshop to handle connections between plugs and slots, and build the routing logic that resolves a requested plug into a secret value using the Provider Registry.

## Context
We have defined how SDKs request secrets (`SEC-001`), how users configure them (`SEC-003`), how providers fetch them (`SEC-004`), and how scripts ask for them (`SEC-002`). 

This task implements the core engine that connects these pieces. When an SDK calls `workshopctl get-secret aws-credentials`, the routing engine must:
1. Look up the `aws-credentials` plug for the calling SDK.
2. Find the `secret` slot it is connected to (e.g., `system:aws-creds-provider`).
3. Extract the `provider` and `source` from that slot.
4. Look up the provider in the `secrets.Registry`.
5. Call `Resolve()` and return the value.

## Scope of Work

1. **Interface Implementation (`internal/interfaces/builtin/secret.go`):**
   - Implement the standard Workshop interface backend for `secret`.
   - **Sanitize/Validate:** Ensure plugs and slots have the correct attributes (leveraging work from SEC-001 and SEC-003).
   - **Connect (`MountConnectedPlug`):** Implement the `MountConnectedPlug` method. This method must verify that the requested secret actually exists in the daemon's memory store (via the provider registry). If it does not exist, return an error so the connection is marked as failed/waiting.
   - **Disconnect Cleanup:** When a `secret` interface is disconnected (e.g., via `workshop disconnect`), the interface backend must instruct the memory provider/registry to purge the cached secret value for that specific connection to maintain security hygiene.

2. **The Resolver Service (`internal/secrets/resolver.go`):**
   - Create a `Resolver` struct or function (e.g., `ResolvePlug(ctx, workshopState, sdkName, plugName) (string, error)`).
   - Implement the traversal logic:
     - Check if the plug exists.
     - Check if the plug is connected to a slot. If not, return a clear `ErrUnroutedSecret` error.
     - Get the slot definition to find the `provider` and `source`.
     - Fetch the provider from the `secrets.Registry`.
     - Call `provider.Resolve(ctx, source)`.

3. **Integration with `workshopctl` (Update SEC-002):**
   - Update the `get-secret` command in `internal/overlord/hookstate/ctlcmd/secret.go`.
   - Replace the stub resolution logic (from SEC-002) with a call to the new `Resolver`.
   - Ensure errors (unrouted, provider not found, access denied) are gracefully caught and printed to `stderr` with appropriate exit codes.

## Acceptance Criteria
- [x] The `secret` interface is registered in `internal/interfaces/builtin/`.
- [ ] `workshop launch` successfully records a connection between a `secret` plug and a `secret` slot without errors.
- [ ] `workshopctl get-secret <plug>` successfully traverses the connection, invokes the provider, and returns the value to `stdout`.
- [x] Requesting an unconnected plug returns a specific error to `stderr` (e.g., "secret plug 'X' is not connected to a slot").
- [ ] Disconnecting a secret plug successfully purges the value from the daemon's memory.
- [x] Unit tests are added for the `Resolver` logic, testing connected, unconnected, and missing provider scenarios.

## Out of Scope
- Automatic environment variable injection (delivering secrets to hooks automatically without `workshopctl`).

## Implementation Status

Landed:

- `internal/secrets/resolver.go` introduces `Resolver`, `ResolvePlug`,
  `Bind`, `SecretResolverFunc`, and the `ErrUnroutedSecret` sentinel.
  `ResolvePlug` checks plug existence, looks up the single connected
  slot, reads its `provider` and `source` attributes, fetches the
  provider via the injected `ProviderLookup`, and returns the resolved
  value. Unrouted, missing-plug, multi-slot, missing-provider, and
  provider-resolution errors are all wrapped with distinguishable
  messages. The sentinel makes the unrouted case detectable via
  `errors.Is`.
- `internal/interfaces/builtin/secret.go` implements the `secret`
  interface: `BeforePreparePlug` / `BeforePrepareSlot` validate
  attributes; `MountConnectedPlug` looks up the provider in the
  registry (via the mockable `GetSecretProvider` seam) and eagerly
  calls `provider.Resolve` so a missing provider or unresolvable
  `source` is reported at connect time rather than at first
  `get-secret`. `AutoConnect` returns `true` to delegate policy to
  the base declaration's `deny-auto-connection`.
- `internal/overlord/hookstate/ctlcmd/secret.go` replaces the SEC-002
  stub: `get-secret` pulls a `SecretResolverFunc` from the hook
  context cache under `"secret-resolver"`, invokes it with the
  requested plug name, prints the value to stdout without a trailing
  newline, and surfaces `ErrUnroutedSecret` with a user-facing
  message that names the plug.
- `internal/daemon/api_workshopctl.go` binds a fresh
  `secrets.Resolver` per `workshopctl` HTTP call (`injectSecretResolver`)
  using the hook context's SDK and the task's project/workshop, and
  caches it under `"secret-resolver"` before dispatching to
  `ctlcmd.Run`.
- Tests:
  - `internal/secrets/resolver_test.go` covers connected,
    unconnected, missing-provider, and unknown-plug paths.
  - `internal/interfaces/builtin/secret_test.go` covers
    `MountConnectedPlug` success, missing provider, and source that
    fails to resolve, using `secrets/builtin.MockProvider` as the
    seam.
  - `internal/overlord/hookstate/ctlcmd/secret_test.go` covers
    success, no-trailing-newline contract, missing context, missing
    positional, non-root execution, the unrouted error path, and a
    no-resolver-in-context error.

Deferred:

- **Disconnect cache purge.** The acceptance criterion assumes a
  daemon-side cache of resolved secret values that the interface
  backend would invalidate on disconnect. The resolver is currently
  lazy — every `get-secret` calls `provider.Resolve` on demand — and
  no provider in tree caches its output, so there is nothing to
  purge. Once a caching provider lands (or the architecture moves to
  eager resolution at connect time), an optional `Purger` extension
  on `secrets.Provider` plus a disconnect hook on `secretInterface`
  can be added. Until then this criterion is intentionally unticked.
- **End-to-end `workshop launch` and `workshopctl get-secret`
  acceptance.** The wiring is in place, but exercising it
  end-to-end requires a registered `host-env` (or other) provider,
  which is owned by SEC-012. Once SEC-012 lands and registers a
  provider in the `secrets/builtin` registry, these two criteria
  can be verified and ticked without further code changes here.