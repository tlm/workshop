# Task Spec: Implement `secret` Slot Interface in `workshop.yaml`

**Task ID:** SEC-003
**Role:** Backend Go Developer
**Status:** Ready for Dev

## Objective
Extend the `workshop.yaml` parser and internal model to support defining `secret` slots. This allows users to configure how a secret plug is fulfilled by a specific provider (e.g., `host-env`).

## Context
While SDKs declare their need for a secret via a `plug` in `sdkcraft.yaml`, the actual fulfillment (the "route" or "provider configuration") is defined as a `slot` in the project's `workshop.yaml`. 

Because Workshop treats the host machine as the `system` SDK, users will typically define secret slots under the `system` SDK block.

Example of the desired `workshop.yaml` addition:
```yaml
name: my-project
sdks:
  # The host system acts as the provider
  - name: system
    slots:
      aws-creds-provider:
        interface: secret
        provider: host-env
        source: AWS_ACCESS_KEY_ID
        
  - name: my-sdk
    plugs:
      aws-credentials:
        bind: system:aws-creds-provider
```

## Scope of Work

1. **Go Model Update:**
   - Locate the internal Go structs responsible for parsing `workshop.yaml` slots (e.g., `internal/workshop/workshop_file.go` and `internal/sdk/sdk.go`).
   - Register the `secret` interface for slots so the parser recognizes it.
   - Ensure the YAML parser correctly unmarshals the `secret` slot attributes:
     - `provider` (string): The type of provider (e.g., `host-env`).
     - `source` (string): The provider-specific reference (e.g., the name of the environment variable).

2. **Validation Logic:**
   - Add validation to ensure that if a slot has `interface: secret`, it must have a valid `provider` and `source`.
   - For the MVP, restrict the allowed `provider` value to `host-env`. Return a validation error if an unsupported provider is specified.

3. **Binding/Connection:**
   - Ensure the existing `bind` and `connections` logic in Workshop allows a `secret` plug to connect to a `secret` slot.
   - Ensure type-checking prevents a `secret` plug from being bound to a non-secret slot (like a `mount` slot).

## Acceptance Criteria
- [ ] Workshop successfully parses a `workshop.yaml` containing a `secret` slot without throwing an "unknown interface" error.
- [ ] Workshop validates that `secret` slots contain the required `provider` and `source` fields.
- [ ] Workshop successfully parses a `bind` or `connection` between a `secret` plug and a `secret` slot.
- [ ] Unit tests are added/updated to verify parsing and validation of the `secret` slot in `workshop.yaml`.

## Out of Scope
- Implementing the actual runtime resolution of the secret (fetching the env var).
- Delivering the secret to the SDK.