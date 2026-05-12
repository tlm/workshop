# Task Spec: Implement CLI UX for Unrouted Secrets

**Task ID:** SEC-006
**Role:** CLI / Go Developer
**Status:** Ready for Dev

## Objective
Update the `workshop connections` and `workshop info` commands to provide clear visibility into the routing state of `secret` plugs, allowing users to easily identify unrouted secrets.

## Context
According to the Secret Routing Model, `workshop launch` does not block if a secret requirement (plug) is unrouted. Instead, the SDK is expected to handle the missing secret at runtime (e.g., by failing a hook or reporting a "waiting" health status). 

To help users debug these situations, Workshop must clearly report which secrets are declared by SDKs and whether they are currently routed to a provider slot.

## Scope of Work

1. **Update `workshop connections`:**
   - Ensure that `secret` interfaces are processed and displayed by the `workshop connections` command.
   - **Routed Secrets:** Should display the SDK's plug and the connected slot (e.g., `system:aws-creds-provider`).
   - **Unrouted Secrets:** When a user runs `workshop connections --all`, unrouted `secret` plugs must be listed with an empty `Slot` column, matching the existing behavior for disconnected plugs.

2. **Update `workshop info`:**
   - `workshop info` currently outputs YAML containing workshop status and SDK details (including connected mount plugs).
   - Extend the `workshop info` output to include a `secrets` or `plugs` section under each SDK.
   - For each declared `secret` plug, output its routing status. 
   - Example desired YAML output snippet:
     ```yaml
     sdks:
       - name: my-sdk
         # ... existing fields ...
         secrets:
           aws-credentials:
             routed: true
             slot: system:aws-creds-provider
           api-token:
             routed: false
     ```

3. **Client/Daemon API Updates:**
   - Ensure the daemon API endpoints that serve `workshop info` and `workshop connections` (e.g., `/v1/workshops/{name}` and `/v1/connections`) correctly serialize the state of `secret` interfaces.

## Acceptance Criteria
- [ ] Running `workshop connections --all` displays unrouted `secret` plugs.
- [ ] Running `workshop connections` displays routed `secret` plugs and their target slots.
- [ ] Running `workshop info <workshop-name>` includes the routing status of `secret` plugs in the YAML output.
- [ ] Integration tests are updated to verify the CLI output for both routed and unrouted secret scenarios.

## Out of Scope
- Emitting warnings during `workshop launch` (launch should remain silent regarding unrouted secrets to preserve the default UX).
- Implementing the `workshopctl get-secret` command (handled in SEC-002).