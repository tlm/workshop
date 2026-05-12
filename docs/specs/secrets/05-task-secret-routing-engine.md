# Task Spec: Implement Secret Routing & Connection Engine

**Task ID:** SEC-005
**Role:** Backend Go Developer
**Status:** Ready for Dev

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
- [ ] The `secret` interface is registered in `internal/interfaces/builtin/`.
- [ ] `workshop launch` successfully records a connection between a `secret` plug and a `secret` slot without errors.
- [ ] `workshopctl get-secret <plug>` successfully traverses the connection, invokes the provider, and returns the value to `stdout`.
- [ ] Requesting an unconnected plug returns a specific error to `stderr` (e.g., "secret plug 'X' is not connected to a slot").
- [ ] Disconnecting a secret plug successfully purges the value from the daemon's memory.
- [ ] Unit tests are added for the `Resolver` logic, testing connected, unconnected, and missing provider scenarios.

## Out of Scope
- Automatic environment variable injection (delivering secrets to hooks automatically without `workshopctl`).