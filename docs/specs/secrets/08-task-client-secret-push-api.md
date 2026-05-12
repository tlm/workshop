# Task Spec: Client-to-Daemon Secret Push API

**Task ID:** SEC-007
**Role:** CLI / Backend Go Developer
**Status:** Ready for Dev

## Objective
Implement the mechanism for the `workshop` CLI to resolve secrets from the user's local environment (`host-env`) or local files (`host-file`) and securely transmit them to the `workshopd` daemon during launch and connection operations.

## Context
Because `workshopd` runs as a background daemon, it cannot access the user's terminal environment variables or desktop keychain. Therefore, the `workshop` CLI (which runs in the user's active session) must act as the resolver for host-backed secrets. 

The CLI will resolve these secrets and push them to the daemon. The daemon will hold these values in memory, acting as a shim provider, so that when an SDK requests the secret via `workshopctl get-secret`, the daemon can fulfill the request.

*Note on Daemon Restarts:* Because secrets are stored in memory to prevent writing them to disk, they will be lost if the `workshopd` daemon restarts. This is a known design limitation for the MVP. Users will need to run `workshop refresh` to re-push secrets if the daemon restarts.

## Scope of Work

1. **CLI Resolution Logic (`cmd/workshop/secrets.go`):**
   - Create a helper function in the CLI that parses `workshop.yaml` for `secret` slots.
   - **`host-env` Provider:** Read the specified `source` from the CLI's environment (`os.Getenv`).
   - **`host-file` Provider:** Read the contents of the file specified in `source` (`os.ReadFile`). Strip any trailing newlines from the file content.
   - If the environment variable or file is missing, log a warning to the user but do not fail the launch (preserving the "unrouted" UX).

2. **API Payload Updates (`internal/daemon/api_workshops.go` & `client/`):**
   - Update the `WorkshopActionSetup` struct (used for `launch` and `refresh`) to include a new field: `Secrets map[string]string`. 
   - The key should be the slot identifier (e.g., `system:aws-creds-provider`), and the value is the resolved secret.
   - Update the `client.Launch` and `client.Refresh` methods to accept and transmit this map.

3. **Daemon In-Memory Storage (`internal/overlord/workshopstate/`):**
   - When the daemon receives the `Secrets` map during a launch/refresh API call, it must store these values securely in memory.
   - **Crucial Security Requirement:** These values MUST NOT be written to the `state.json` file on disk. They should be attached to the runtime context or an in-memory map tied to the active workshop instance.

4. **Daemon Provider Shim (`internal/secrets/providers/memory.go`):**
   - Implement a `memory` provider that satisfies the `secrets.Provider` interface.
   - When the routing engine (SEC-005) asks to resolve a `host-env` or `host-file` slot, the daemon will use this shim to look up the value that was pushed by the CLI and stored in memory.

## Acceptance Criteria
- [ ] The CLI successfully reads a required environment variable and includes it in the `/v1/workshops` POST payload.
- [ ] The CLI successfully reads a local file and includes its contents in the POST payload.
- [ ] The daemon receives the secret and stores it in memory without persisting it to disk.
- [ ] The daemon's routing engine successfully retrieves the pushed secret using the memory shim provider.
- [ ] If the CLI cannot find the environment variable or file, the launch proceeds, and the secret remains unrouted.
- [ ] Unit tests verify that secrets are stripped or ignored during state serialization to disk.

## Out of Scope
- Implementing the `host-keychain` provider (deferred to SEC-07).
- Delivering secrets automatically as environment variables to hooks (this is pull-based via `workshopctl` for now).