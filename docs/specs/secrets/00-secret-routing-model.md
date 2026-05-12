# Secret Routing Model

## Status

Draft for hackathon planning.

## Purpose

Workshop should let SDKs declare the secrets they know how to consume while
leaving users in control of whether those secrets are provided, where they come
from, and when they are exposed.

Workshop is a gateway for secret resolution and delivery. It is not the system
of record for secret values.

## Goals

- Let SDKs declare named secret requirements such as `FOO`.
- Preserve the current `workshop launch` experience: launch must not require
  extra secret setup by default.
- Let users optionally route declared secrets to local or external secret
  sources.
- Let users add or change secret routes after launch.
- Support local user configuration for repeated automatic routing.
- Deliver resolved secret values just in time to the SDK process that needs
  them.
- Keep raw secret values out of persisted Workshop state, project files, logs,
  task summaries, and command output.

## Non-goals

- Workshop does not become a general-purpose secrets manager.
- Workshop does not persist raw secret values.
- The first implementation does not need to support every provider class.
- The first implementation does not need dynamic Vault-style leases, rotation,
  or revocation.
- SDKs do not choose the user's secret provider or host-specific source path.

## Core Model

The model has four contracts:

- `SecretRequirement`: declared by an SDK. It describes a secret the SDK can
  consume.
- `SecretRoute`: configured by the user or local project state. It maps a
  requirement to a provider-backed source.
- `SecretProvider`: implemented by Workshop or an integration. It resolves a
  route into a value at runtime.
- `SecretDelivery`: selected by Workshop. It describes how the resolved value
  is exposed to the SDK process.

The important separation is:

- SDKs declare what they can consume.
- Users configure where values come from.
- Workshop resolves and delivers values without owning them.

## Default UX

`workshop launch` must continue to work without secret-specific flags or prior
setup.

If an SDK declares a secret requirement and no route exists:

- Workshop does not inject the secret.
- Workshop does not block launch by default.
- The SDK may fail, wait, or report degraded health through its existing hooks.
- Workshop should be able to report that a declared secret is currently
  unrouted.

This keeps secret wiring opt-in while giving users enough visibility to fix
missing credentials.

## User Wiring UX

Users should be able to wire secrets in three ways.

### During Launch

A user may provide one or more routes as part of launch.

Example command shape:

```text
workshop launch --secret aws.FOO=host-env:FOO
```

This is a convenience path. It should not be the only way to configure secrets.

### After Launch

A user may add, replace, inspect, or remove routes after a workshop exists.

Routes are expressed as standard plug/slot connections, so the existing
Workshop CLI surface is reused rather than introducing a dedicated
`workshop secrets` command tree. Users edit slots in `workshop.yaml` and
manage their connections via the regular `workshop connect` /
`workshop disconnect` / `workshop connections` commands.

After a route changes, Workshop should make the new route available to future
secret deliveries. A later spec should define whether route changes can trigger
hook reruns, health checks, service restarts, or refresh operations.

### Local Automatic Routing

A user may define local routes that are applied automatically when a matching
SDK requirement is present.

The local routing file is user-owned and host-specific. It must not be treated
as a portable project definition, and it must not contain raw secret values.

Example local route shape:

```yaml
secret-routes:
  - sdk: aws
    secret: FOO
    provider: host-env
    source: FOO
```

## Contract: SecretRequirement

A `SecretRequirement` is declared by an SDK. It is metadata, not authorization.

Proposed shape:

```yaml
secrets:
  - name: FOO
    description: Token used by the SDK to call service X.
```

Fields:

- `name`: stable SDK-local name for the requirement.
- `description`: user-facing explanation of why the SDK wants the secret.

Rules:

- Secret names are scoped to the SDK that declares them.
- The canonical requirement identifier is `<sdk>.<name>`, for example
  `aws.FOO`.
- SDKs must not declare host-specific provider names, file paths, account IDs,
  or vault paths as requirements.

## Contract: SecretRoute

A `SecretRoute` maps a requirement to a provider source.

Proposed shape:

```yaml
sdk: aws
secret: FOO
provider: host-env
source: FOO
```

Fields:

- `sdk`: SDK name that owns the requirement.
- `secret`: requirement name declared by the SDK.
- `provider`: provider type or provider instance name.
- `source`: provider-specific reference to the secret.

Rules:

- Routes may be stored in local user or local project configuration.
- Routes may be stored in Workshop state as metadata and references.
- Routes must not store raw secret values.
- A route is valid only if the target SDK requirement exists or the user
  explicitly allows a pending route for a future SDK.
- Provider-specific `source` data must be treated as sensitive metadata even
  when it is not itself a secret value.

## Contract: SecretProvider

A `SecretProvider` resolves a route at runtime.

Conceptual interface:

```text
Resolve(route, context) -> SecretValue | SecretUnavailable | SecretDenied
```

Provider responsibilities:

- Validate that the route source is well-formed for the provider.
- Resolve the value only when Workshop is about to deliver it.
- Return errors that distinguish missing values, denied access, and provider
  failures.
- Avoid logging raw values.

Initial provider candidates:

- `host-env`: read a value from the host process environment.
- `local-file`: read a selected host file or config fragment.

Later provider candidates:

- Host keychain.
- 1Password.
- Bitwarden.
- Vault or another dynamic credential service.

## Contract: SecretDelivery

`SecretDelivery` describes how Workshop exposes a resolved value to an SDK
process.

The MVP supports a single delivery mode: pull-based resolution via
`workshopctl get-secret <plug>`. An SDK hook or script asks the daemon for a
resolved value at the moment it needs one, and the daemon returns the value
over the authenticated `workshopctl` socket. The SDK is then responsible for
using the value (for example, by exporting it as an environment variable for
the duration of the script).

Rules for pull delivery:

- Workshop resolves the value only when the SDK explicitly requests it.
- Workshop must avoid including resolved values in task logs, command logs,
  debug output, or persisted state.
- The value lifetime is bounded by the requesting process unless a provider
  imposes a shorter lifetime.

Future delivery modes may include:

- Automatic environment variable injection into specific hooks or actions
  (see `09-future-todos.md`).
- Files mounted into the workshop for the duration of a process.
- File content written to a temporary path with restricted permissions.
- Agent sockets or provider sockets.
- Native systemd credentials for in-workshop services.

## Runtime Flow

For a process that supports secret delivery:

1. An SDK hook or script invokes `workshopctl get-secret <plug>`.
2. Workshop identifies the calling SDK from the `workshopctl` context.
3. Workshop loads the SDK's `SecretRequirement` for that plug.
4. Workshop finds the matching `SecretRoute` (the connected slot) from
   `workshop.yaml` and persisted route metadata.
5. Workshop resolves the route through its `SecretProvider`.
6. Workshop returns the resolved value to the caller over the `workshopctl`
   socket.
7. Workshop discards the resolved value after delivery.

For the MVP, target processes are expected to be SDK hooks, workshop actions,
or commands started through `workshop run` and `workshop exec` that
explicitly call `workshopctl get-secret`.

## Missing, Denied, and Failed Secrets

Missing route:

- No route exists for a declared requirement.
- Workshop does not deliver a value.
- Launch continues by default.
- Workshop should report the requirement as unrouted.

Denied route:

- A route exists, but the provider or user policy denies access.
- Workshop does not deliver a value.
- The target process should not receive a placeholder value.
- Workshop should report a denial without exposing secret metadata beyond what
  is needed for troubleshooting.

Provider failure:

- A route exists, but the provider cannot resolve it due to an operational
  error.
- Workshop does not deliver a value.
- The command or hook behavior depends on the operation type and later policy
  decisions.

## Security Boundary

Workshop may persist:

- SDK secret requirement metadata.
- User route metadata.
- Provider configuration that does not contain raw secret values.
- Audit metadata such as which route was used, when, for which SDK, and whether
  resolution succeeded.

Workshop must not persist:

- Raw secret values.
- Environment maps containing resolved secret values.
- Full command lines if they contain resolved secret values.
- Provider responses that include secret material.

Workshop must avoid:

- Printing secret values in CLI output.
- Storing secret values in task logs or hook logs.
- Sending secret values to unrelated SDKs or processes.
- Automatically routing a secret solely because the SDK declared a matching
  name.

## Open Questions

- Which operations should support secret delivery in the first implementation:
  hooks only, actions only, `workshop exec`, or all three?
- Where should local automatic routing live on disk?
- Should routes be workshop-scoped, project-scoped, user-scoped, or support all
  three?
- How should users inspect missing requirements without exposing sensitive
  provider metadata?

## MVP Recommendation

The first implementation should support:

- SDK-declared secret requirements.
- User-managed routes stored as metadata.
- A `host-env` provider.
- Pull-based delivery via `workshopctl get-secret`.
- Non-blocking `workshop launch` when requirements are unrouted.
- A way to list declared, routed, and unrouted secrets for a workshop.

The first demo should show:

1. An SDK declares `FOO`.
2. `workshop launch` succeeds without wiring `FOO`.
3. Workshop reports `FOO` as unrouted.
4. The user connects `aws.FOO` to a host source.
5. A later SDK hook, action, or command retrieves `FOO` via
   `workshopctl get-secret`.
6. Workshop does not persist or print the resolved value.
