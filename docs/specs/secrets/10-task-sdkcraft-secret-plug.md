# Task Spec: Update `sdkcraft` to support `secret` interface

**Task ID:** SEC-010
**Role:** Python Developer
**Status:** Ready for Dev

## Objective
Update the `sdkcraft` CLI tool to recognize and validate the new `secret` plug interface in `sdkcraft.yaml`.

## Context
We are introducing a new `secret` interface for Workshop SDKs to declare their secret requirements. SDK developers define these requirements in their `sdkcraft.yaml` under the `plugs` section. The `sdkcraft` CLI tool is responsible for parsing, validating, and packing this YAML. We need to update its Pydantic models to recognize `interface: secret`.

Example of the desired `sdkcraft.yaml` addition:
```yaml
plugs:
  aws-credentials:
    interface: secret
    description: "AWS credentials required to provision cloud resources"
```

## Scope of Work

1. **Update Project Models (`sdkcraft/models/project.py`):**
   - Create a new `SecretPlug` class inheriting from `models.CraftBaseModel`.
   - Define the `interface` field as `Literal["secret"]`.
   - Define an optional `description` field (`description: str | None = None`).
   - Add `SecretPlug` to the `Plug` type union (`type Plug = Annotated[...]`).

2. **Update Marked Project Models (`sdkcraft/models/marked_project.py`):**
   - Update the `OtherPlug` class's `interface` Literal to include `"secret"`.
   - Ensure `OtherPlug` allows the `description` field or inherits it correctly.

3. **Validation:**
   - Unlike `ssh-agent` or `camera`, `secret` plugs do not have naming restrictions (e.g., the plug name can be `aws-credentials`, not just `secret`). Ensure no `validate_policy` restricts the plug name.

4. **Tests:**
   - Add unit tests in the `tests/` directory to verify that `sdkcraft` can successfully parse an `sdkcraft.yaml` containing a `secret` plug.

## Acceptance Criteria
- [ ] `sdkcraft pack` successfully parses an `sdkcraft.yaml` with a `secret` plug without validation errors.
- [ ] The `secret` plug allows an optional `description` field.
- [ ] Unit tests pass for parsing the `secret` plug.

## Out of Scope
- Implementing the `secret` interface in the Workshop daemon (handled in other specs).