# Task Spec: Implement `host-file` Secret Provider (Daemon-Side, Guarded Root Read)

**Task ID:** SEC-012
**Role:** Backend Go Developer
**Status:** Draft
**Depends on:** SEC-004 (provider interface), SEC-005 (routing engine)
**Amends:** SEC-008 (host-file is removed from SEC-008's scope; SEC-008 covers `host-env` only)

## Objective

Add a `host-file` provider to the `system` SDK so users can route a `secret`
plug to a file on the workshop owner's host filesystem. The daemon performs
the read directly (not the CLI), so the value survives `workshopd` restarts
and reflects on-disk rotation without `workshop refresh`.

The daemon reads the file as `root` but applies strict, defense-in-depth
guards so a misconfigured or malicious slot cannot read arbitrary files on
behalf of the workshop owner.

## Context

`host-env` (SEC-008) follows a client-side push model: the CLI reads the
user's environment, pushes values to the daemon, the daemon caches them in
memory, and the cache is lost on daemon restart. That trade-off makes sense
for environment variables — they exist only in the user's shell.

Files do not have that constraint. A daemon-side `host-file` provider gives
us three properties the push model cannot:

- The secret value survives a `workshopd` restart (the file is still on disk).
- Rotation just works: the next `workshopctl get-secret` re-reads the file.
- No CLI changes are required to support it.

The trade-off is that the daemon (running as root) must read a file that
belongs to an unprivileged user. We accept this by reading as root with
strict guards rather than introducing a new host-side privilege-drop
primitive (which Workshop does not currently have — its "do work as the
user" pattern is in-container LXD exec, which does not apply here).

## Slot Configuration

The slot shape is unchanged from the routing model:

```yaml
sdks:
  - name: system
    slots:
      aws-creds-provider:
        interface: secret
        provider: host-file
        source: ~/.aws/credentials
```

Fields:

- `provider: host-file`
- `source`: a path on the workshop owner's filesystem. May start with `~` or
  `$HOME` to mean the owner's home directory. The slot stores the path
  string verbatim — expansion happens at resolve time using the workshop
  owner's identity, not the daemon's.

## Resolution Model

Resolution is **daemon-side and lazy**:

1. An SDK hook calls `workshopctl get-secret <plug>`.
2. The SEC-005 resolver finds the connected slot, sees `provider: host-file`,
   and dispatches to the `host-file` provider in the registry.
3. The provider reads the file (with the guards below) and returns the
   value to the resolver.
4. The resolver returns the value across the `workshopctl` socket.
5. The value is discarded; no caching, no persistence.

No in-memory cache is introduced. If hot-loop callers create measurable
overhead later, a short-lived (e.g. 5s) in-process TTL cache can be added
without changing the contract.

## Security Guards

The provider runs as root inside the daemon. To prevent it from reading
files the workshop owner could not read themselves, **every** resolution
must enforce all of the following, in order, before returning a value:

1. **Resolve the workshop owner.** The resolver passes the owner's UID, GID,
   and home directory to the provider (see "Provider Contract" below). If
   the owner cannot be determined, return `ErrSecretDenied`.
2. **Expand the path** under the owner's home, not the daemon's. `~` and
   `$HOME` resolve to the owner's home directory. After expansion the path
   must be absolute; reject otherwise.
3. **Canonicalize the path** (resolve `..`, normalise) and confirm it is
   contained under the owner's home directory. Reject paths that escape
   the home directory (e.g. `~/../../etc/shadow` post-`..`-resolution).
4. **`Lstat` the path.** Reject symlinks outright — do not follow them.
   This prevents a user from planting a symlink to `/etc/shadow` inside
   their own home and having the daemon dereference it as root.
5. **Open with `O_NOFOLLOW | O_CLOEXEC`.** Defense in depth against a TOCTOU
   race between the `Lstat` and the open.
6. **Verify `st_uid == owner_uid`** on the opened file descriptor (use
   `Fstat`, not the pre-open `Lstat`, to close the TOCTOU window). Reject
   if the file is not owned by the workshop owner.
7. **Verify the file is a regular file** (`S_ISREG`). Reject devices,
   FIFOs, sockets, directories.
8. **Enforce a size cap** (64 KiB for v1). Refuse files larger than the
   cap before reading the full content into memory.
9. **Read the content**, strip a single trailing `\n` if present, return
   the result.

Each guard failure must map to a clear error. Suggested mapping:

- Path expansion failure, not under home, escape via `..` → `ErrSecretDenied`
  with a message that names the policy violated (do not echo the offending
  path back if it could leak filesystem layout).
- Symlink, wrong owner, non-regular file → `ErrSecretDenied`.
- File not found → `ErrSecretNotFound`.
- Oversized → `ErrSecretDenied` (treat as policy violation, not a generic
  failure).
- I/O failure → generic provider error.

## Provider Contract

The current `secrets.Provider` interface in
`internal/secrets/provider.go` is:

```go
type Provider interface {
    Name() string
    Resolve(ctx context.Context, source string) (string, error)
}
```

`host-file` needs the workshop owner's identity (UID, GID, home directory)
to enforce the guards. Two options:

- **(a)** Pass owner info through `context.Context` via a typed accessor
  (`secrets.WorkshopOwnerFromContext(ctx)`); the SEC-005 resolver populates
  the context before calling `Resolve`. Keeps the interface stable.
- **(b)** Extend the interface to `Resolve(ctx, req ResolveRequest)` where
  `ResolveRequest` carries `Source` and `Owner`. More explicit; requires
  updating the host-env memory shim provider signature too.

This spec **recommends (a)**: it keeps SEC-004's interface intact, host-env
providers ignore the context value, and the dependency is documented at
the resolver→provider boundary. If a later provider needs the same data
the same accessor works.

The workshop owner UID/GID/home is already discoverable via the existing
state path: the daemon stores `user.workshop.username` in the LXD project
config (`internal/workshop/lxd/lxd_backend_project.go`), and
`osutil.UidGid` resolves it to UID/GID. SEC-005's resolver fetches the
owner via this path and attaches it to the context before calling
`provider.Resolve`.

## Scope of Work

1. **Provider implementation (`internal/secrets/builtin/hostfile.go`):**
   - Implement a `hostFileProvider` satisfying `secrets.Provider`.
   - `Name()` returns `"host-file"`.
   - `Resolve` performs all nine guards above in order.
   - Register via `registerProvider` in package init.

2. **Owner context plumbing (`internal/secrets/`):**
   - Add a `WorkshopOwner` struct (`UID uint32`, `GID uint32`, `Home string`)
     and `WithWorkshopOwner` / `WorkshopOwnerFromContext` helpers.
   - The SEC-005 resolver populates the context before dispatching to the
     provider.

3. **Interface backend update
   (`internal/interfaces/builtin/secret.go`):**
   - Add `"host-file"` to `allowedSecretProviders`.
   - Validate that `source` for `host-file` slots is a non-empty string.
     Do not attempt to expand or validate the path at slot-parse time —
     expansion is deferred to resolve time so the slot is portable across
     hosts.

4. **Resolver integration (SEC-005):**
   - Where SEC-005 resolves a slot to a provider call, look up the
     workshop owner and inject it via `WithWorkshopOwner`.
   - Existing `host-env` flow is unaffected (memory shim provider ignores
     the context value).

5. **Tests (`internal/secrets/builtin/hostfile_test.go`):**
   - Happy path: file in `$HOME`, owned by the test user, returns content
     with trailing newline stripped.
   - Symlink rejection (symlink inside `$HOME` to another file in `$HOME`).
   - Wrong-owner rejection (file under `$HOME` chowned to a different UID
     where the test environment permits).
   - Path escape rejection (`~/../etc/passwd`).
   - Non-regular file rejection (FIFO).
   - Oversized file rejection.
   - Missing file → `ErrSecretNotFound`.
   - Missing owner context → `ErrSecretDenied`.

6. **Integration test (`tests/main/interface-secret-host-file/`):**
   - Mirror the structure of the existing `tests/main/interface-secret/`
     test. Create a file in the test user's home, route a slot at it,
     verify `workshopctl get-secret` returns the contents inside the
     workshop.

## Acceptance Criteria

- [ ] `internal/secrets/builtin/hostfile.go` exists and the provider is
      registered.
- [ ] `host-file` is accepted as a slot provider in
      `internal/interfaces/builtin/secret.go`.
- [ ] All nine guards are enforced and individually unit-tested.
- [ ] `workshopctl get-secret <plug>` returns the file contents (trailing
      `\n` stripped) when wired to a valid `host-file` slot.
- [ ] The daemon does not log, persist, or otherwise emit the resolved
      file contents anywhere outside the immediate `workshopctl` response.
- [ ] After a daemon restart, the same `workshopctl get-secret` call still
      succeeds without any `workshop refresh` step.
- [ ] A symlink, wrong-owner, oversized, or out-of-home `source` yields
      a non-zero exit code and a stderr message on the workshopctl side
      without exposing the on-disk content.

## Out of Scope

- File-as-file delivery (writing the content into a tempfile inside the
  workshop with restricted permissions). Tracked under future delivery
  work in [09-future-todos.md](09-future-todos.md).
- Watching the file for changes and pushing updates to hooks. Rotation
  is handled passively via lazy reads.
- Allowing `host-file` `source` paths outside the workshop owner's home
  directory. If a real use case appears (e.g. admin-laid-down
  `/etc/myapp/creds`), it can be enabled later behind an explicit
  per-slot opt-in attribute.
- macOS / Windows host support. The guards assume Linux semantics for
  `Lstat`/`Fstat`/`O_NOFOLLOW`.
- Establishing a host-side privilege-drop primitive (fork as workshop
  owner UID). The guarded root-read model in this spec is the chosen
  alternative. If a future provider genuinely needs to read as the user
  (e.g. requires the user's GPG agent), that primitive can be introduced
  then.

## Open Questions

- Should the size cap (64 KiB) be a constant or a per-slot attribute?
  Constant for v1; revisit if a real use case wants a larger limit.
- Should the provider expose a `workshop info` field indicating whether
  the routed file is currently readable (a preflight indicator)? Likely
  yes, owned by SEC-006 (CLI UX for unrouted secrets), but the daemon
  must expose the preflight result through the existing info payload.
