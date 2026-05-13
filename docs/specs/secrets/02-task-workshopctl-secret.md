# Task Spec: Implement `workshopctl get-secret` command

**Task ID:** SEC-002
**Role:** Backend Go Developer
**Status:** In Progress — plug-declaration validation deferred to SEC-001

## Objective
Add a `get-secret` command to `workshopctl` that allows an SDK running inside the workshop to request the value of a declared `secret` plug.

## Context
SDKs need a way to retrieve the secrets they declared in their `sdkcraft.yaml` at runtime. `workshopctl` acts as the bridge between the SDK environment and the host daemon. To maintain security, the daemon must verify the SDK's identity before handing over any secrets.

## Scope of Work

1. **Command Implementation:**
   - Create `internal/overlord/hookstate/ctlcmd/secret.go`.
   - Implement a `getSecretCommand` struct that satisfies the `command` interface.
   - Register the command as `get-secret` using `addCommand`.
   - Add `get-secret` to the `nonRootAllowed` list in `ctlcmd.go` so SDK scripts can run it without sudo.

2. **Command Logic (`Execute` method):**
   - Accept a single positional argument: `<plug-name>`.
   - **Authentication/Context:** Retrieve the SDK context via `c.ensureContext()`. This step implicitly validates the `WORKSHOP_COOKIE` to ensure the caller is an authenticated SDK hook.
   - **Validation:** Verify that the authenticated SDK has actually declared a plug named `<plug-name>` in its `sdkcraft.yaml`, and that the plug's interface type is `secret`. (Return an error to stderr if not).
   - **Resolution (Stub for now):** Since the routing engine isn't built yet, temporarily return a placeholder string (e.g., `"stub-secret-value-for-" + plugName`) to `stdout` to prove the IPC plumbing works. SEC-005 will replace this stub with the real resolver.

3. **Output Formatting:**
   - Print *only* the secret value to `stdout` upon success.
   - Print all validation or system errors to `stderr`.
   - Return a non-zero exit code on failure.

## Acceptance Criteria
- [x] Running `workshopctl get-secret <plug-name>` inside a workshop hook successfully returns the stub secret to stdout.
- [x] Running the command without a valid `WORKSHOP_COOKIE` (outside a hook context) fails securely.
- [ ] Running the command for a plug that doesn't exist or isn't a `secret` plug returns an error to stderr and exits with code 1.
- [x] The command can be executed by a non-root user inside the workshop.
- [x] Unit tests are added in `internal/overlord/hookstate/ctlcmd/secret_test.go`.

## Implementation Status

Landed:

- `internal/overlord/hookstate/ctlcmd/secret.go` registers a
  `get-secret` subcommand backed by `getSecretCommand`. It takes a
  single required `<plug-name>` positional, calls `c.ensureContext()`
  to enforce the hook-context authentication boundary, and writes
  `stub-secret-value-for-<plug-name>` to stdout with no trailing
  newline.
- `internal/overlord/hookstate/ctlcmd/ctlcmd.go` adds `get-secret` to
  `nonRootAllowed` so SDK hooks running as a non-root user can invoke
  it without sudo.
- `internal/overlord/hookstate/ctlcmd/secret_test.go` covers: stub
  success path, the no-trailing-newline output contract, missing hook
  context, the required positional argument, and non-root execution.

Deferred:

- Validation that the calling SDK has declared a plug named
  `<plug-name>` with `interface: secret`. This depends on SEC-001
  landing the `secret` interface in the Go SDK model so the lookup
  has something to query. Once SEC-001 is in, `Execute` should
  retrieve the SDK from the hook context, look up the plug in its
  parsed `sdkcraft.yaml`, and reject the call with a stderr error
  and non-zero exit code if the plug is missing or not a `secret`
  plug.
- Replacing the stub return value with a real resolver call. This is
  owned by SEC-005 and is explicitly out of scope here.
