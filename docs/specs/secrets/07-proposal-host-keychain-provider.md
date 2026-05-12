# Task Spec: Implement `host-keychain` Secret Provider

**Task ID:** SEC-007 (Updated)
**Role:** CLI / Backend Go Developer
**Status:** Ready for Dev

## Objective
Introduce a `host-keychain` secret provider that allows the `workshop` CLI to securely retrieve secrets directly from the user's native Linux desktop credential store (e.g., GNOME Keyring or KDE KWallet) using the Secret Service API.

## Context
While the `host-env` and `host-file` providers are useful, storing long-lived credentials (like AWS keys or database passwords) in plain text on disk or in environment variables is a security risk. Users typically store these in their OS keychain.

By leveraging the Client-Side Resolution (Push Model) established in `SEC-008`, the `workshop` CLI can query the user's keychain, handle any OS-level prompts, and push the secret to the daemon securely.

Example `workshop.yaml` configuration:
```yaml
sdks:
  - name: system
    slots:
      aws-creds-provider:
        interface: secret
        provider: host-keychain
        # Format: <service>/<account>
        source: "aws-cli/my-prod-account"
```

## Scope of Work

1. **Dependency Addition:**
   - Add `github.com/zalando/go-keyring` to the project's `go.mod`.
   - *Note on CGO:* For Linux targets, this library uses a pure Go D-Bus implementation (`godbus/dbus/v5`). It does **not** require CGO or `libsecret` C headers to compile on Linux.

2. **CLI Resolution Logic (`cmd/workshop/secrets.go`):**
   - Extend the client-side resolution logic to support `provider: host-keychain`.
   - **Addressing UX:** Parse the `source` string as a 2-tuple: `<service>/<account>`. If the string does not contain a `/`, return a validation error explaining the expected format.
   - Call `keyring.Get(service, account)`.
   - **Handling Prompts:** The OS (via Polkit/D-Bus) will automatically handle prompting the user to unlock their keyring if necessary. The Go code simply blocks until the OS returns the secret or an error.
   - If `keyring.Get` returns an error (e.g., `ErrNotFound`, user clicked "Deny", or running in a headless environment without D-Bus), catch the error, log a helpful warning to the user, and leave the secret unrouted. Do not fail the launch.

3. **Daemon Integration:**
   - No changes are required to the daemon. The CLI will push the resolved string to the daemon via the existing `Secrets` map payload. The daemon will treat it exactly like a `host-env` secret and store it in memory.

## Acceptance Criteria
- [ ] The `github.com/zalando/go-keyring` dependency is added.
- [ ] The CLI successfully parses a `<service>/<account>` source string.
- [ ] The CLI successfully retrieves a secret from the Linux Secret Service (GNOME Keyring/KWallet).
- [ ] The OS successfully prompts the user if the keyring is locked.
- [ ] The retrieved secret is pushed to the daemon via the API payload.
- [ ] If the secret is not found or access is denied, the launch proceeds, and the secret remains unrouted.
- [ ] The project still compiles successfully on Linux with `CGO_ENABLED=0`.

## Out of Scope
- macOS and Windows keychain support (the library supports them, but they are out of scope for this Linux-focused MVP and may require CGO/build pipeline changes later).