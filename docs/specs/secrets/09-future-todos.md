# Future TODOs and Explorations

This document tracks future work items, ideas, and extensions for the Workshop Secrets feature that are currently outside the immediate MVP scope but are important for the overall vision.

## 1. Bitwarden SDK Architecture
- Design a `bitwarden-sdk` that acts as a secrets manager inside the workshop.
- Explore bootstrapping this SDK using the `host-file` or `host-env` providers to pass the `BW_SESSION` token (as discussed, this is a great stretch goal for the hackathon).
- Prove that Workshop can support complex, containerized secret management workflows without compromising host security.

## 2. Daemon Restart UX
- Currently, in-memory secrets are lost if the `workshopd` daemon restarts.
- Explore UX improvements for this scenario. For example, if an SDK requests a routed secret that is no longer in memory, `workshopctl` could return a specific error code that prompts the user to run `workshop refresh` to re-push their secrets.

## 3. Secret Rotation / Refresh Hooks
- If a user updates their `host-env` secret and runs `workshop refresh`, the daemon's memory is updated.
- Explore whether `workshop refresh` should automatically trigger specific SDK hooks (e.g., a `secrets-changed` hook) so the SDK knows to fetch the new value, rather than waiting for the next time the SDK happens to run a script.

## 4. Environment Variable Delivery (Pull vs Push)
- The MVP relies on `workshopctl get-secret` (Pull).
- Explore "Push" delivery, where Workshop automatically injects resolved secrets into the environment variables of specific hooks or actions before they start, removing the need for the SDK to call `workshopctl`.

## 5. Systemd Credentials Integration
- For background services running via systemd inside the workshop, `workshopctl get-secret` is not ideal.
- Explore exposing resolved secrets natively using systemd's credentials system by setting `CREDENTIALS_DIRECTORY=/run/host/credentials` and securely placing the secrets there for PID 1 to distribute to services via `LoadCredential=`.

## 6. Secret Service API Emulation (D-Bus)
- Instead of forcing SDKs to use `workshopctl`, explore implementing the Linux Secret Service API over the untrusted `workshopctl` socket.
- This would allow standard tools inside the workshop (like Python's `keyring` library or the `secret-tool` CLI) to "just work" by treating Workshop as if it were a native GNOME Keyring/KWallet.

## 7. Bi-directional Secrets (Write to Host)
- The current architecture is strictly Read-Only (Host -> Workshop).
- Explore allowing apps inside a workshop to store project-scoped secrets back into the host's keyring (e.g., after completing an OAuth device flow inside the workshop, saving the token so it survives a workshop rebuild).

## 8. Ad-hoc Secret Injection
- Explore adding a `--secret` flag to `workshop exec` and `workshop run` (e.g., `workshop exec --secret AWS_TOKEN=my-key -- aws s3 ls`).
- This would allow users to inject temporary secrets for debugging without needing to formally declare them in `sdkcraft.yaml` or route them in `workshop.yaml`.