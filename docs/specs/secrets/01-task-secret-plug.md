# Task Spec: Implement `secret` Plug Interface in `sdkcraft.yaml`

**Task ID:** SEC-001
**Role:** Backend Go Developer
**Status:** Done

## Objective
Extend the Workshop SDK definition model to support a new plug interface named `secret`. This task is strictly limited to parsing, validating, and representing the `secret` plug in memory. It does *not* include the actual routing or delivery of the secret.

## Context
SDKs need a declarative way to request secrets from the Workshop environment. We are introducing a `secret` plug to `sdkcraft.yaml`. When an SDK declares this plug, it signals an expectation to consume a secret. 

Example of the desired `sdkcraft.yaml` addition:
```yaml
plugs:
  aws-credentials:
    interface: secret
    description: "AWS credentials required to provision cloud resources"
```

## Scope of Work (Workshop Core)

1. **Go Model Update:**
   - Locate the internal Go structs responsible for parsing `sdkcraft.yaml` plugs.
   - Register/implement the `secret` plug interface type so the parser recognizes it as a valid interface.
   - Ensure the YAML parser correctly unmarshals the `secret` plug and any standard base plug attributes (like `description`) into the internal representation.

2. **Validation Logic:**
   - Add basic validation to ensure the `secret` plug is processed correctly during the `sdkcraft pack` or `workshop launch` validation phases.
   - Ensure no unsupported attributes cause silent failures.

## Acceptance Criteria
- [ ] Workshop successfully parses an `sdkcraft.yaml` containing a valid `secret` plug without throwing an "unknown interface" error.
- [ ] The parsed `secret` plug data is correctly populated in the internal Go SDK model, ready for the secret routing engine to consume later.
- [ ] Unit tests are added/updated to verify parsing and validation of the `secret` plug.

## Out of Scope
- Updating project documentation or JSON schemas (e.g., `schema-sdkcraft.json`).
- Implementing the secret providers (e.g., Host Keychain, Vault).
- Implementing the routing logic that matches this plug to a provider.
- Delivering the secret to the SDK at runtime.
