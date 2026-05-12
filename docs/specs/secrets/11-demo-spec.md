# Hackathon Demo Spec: Secure AWS Provisioning

**Task ID:** SEC-011
**Role:** Presenter / Developer
**Status:** Ready for Demo

## Objective
Create a cohesive narrative and script for the hackathon presentation demonstrating the end-to-end Workshop Secrets feature. The demo will showcase securely provisioning AWS resources inside a Workshop without ever writing keys to disk.

## Scenario
A developer wants to use an `aws-sdk` inside their Workshop to provision cloud resources. They need to pass their AWS credentials to the SDK, but they do not want to hardcode them in a configuration file or leave them lying around on disk.

## Demo Script & Flow

### 1. The SDK Declaration (`sdkcraft.yaml`)
**Action:** Show the `sdkcraft.yaml` for the `aws-sdk`.
**Talking Track:** "Here we have an AWS SDK. It declares that it needs AWS credentials to function. It does this by defining a `secret` plug. Notice that the SDK doesn't care *where* the secret comes from, it just declares what it needs."
```yaml
name: aws-sdk
plugs:
  aws-credentials:
    interface: secret
    description: "AWS credentials required to provision cloud resources"
```

### 2. The User Configuration (`workshop.yaml`)
**Action:** Show the project's `workshop.yaml`.
**Talking Track:** "As a user, I want to use this SDK. I configure my `workshop.yaml` to route the SDK's requested secret to my host machine's environment variables using the `host-env` provider."
```yaml
name: secure-aws-project
sdks:
  - name: system
    slots:
      host-aws-creds:
        interface: secret
        provider: host-env
        source: AWS_ACCESS_KEY_ID
  - name: aws-sdk
    plugs:
      aws-credentials:
        bind: system:host-aws-creds
```

### 3. The Launch (Client-to-Daemon Push)
**Action:** In the terminal, export the environment variable and run `workshop launch`.
```bash
export AWS_ACCESS_KEY_ID="AKIAIOSFODNN7EXAMPLE"
workshop launch
```
**Talking Track:** "When I run `workshop launch`, the CLI reads the environment variable from my active terminal session and securely pushes it to the Workshop daemon. The daemon holds this secret entirely in memory—it is never written to `state.json` or any disk storage."

### 4. The SDK Execution (`workshopctl get-secret`)
**Action:** Show the setup hook script inside the `aws-sdk`.
**Talking Track:** "Inside the isolated Workshop container, the SDK's setup hook runs. It uses `workshopctl get-secret` to request the value. The daemon verifies the SDK's identity and returns the secret from memory."
```bash
#!/bin/bash
# Inside aws-sdk/hooks/setup
export AWS_ACCESS_KEY_ID=$(workshopctl get-secret aws-credentials)

# Use the credentials to verify identity
aws sts get-caller-identity
```

### 5. The Security Proof
**Action:** Run `workshop info` and inspect the host filesystem.
**Talking Track:** "We can run `workshop info` to see that the secret is successfully routed. More importantly, if we inspect the Workshop daemon's state files on the host, the secret is nowhere to be found. If the daemon restarts, the memory is wiped, ensuring perfect security hygiene."

## Preparation Checklist
- [ ] Build the `aws-sdk` with the `secret` plug and setup hook.
- [ ] Create the `secure-aws-project` directory with the `workshop.yaml`.
- [ ] Ensure the Workshop binary is compiled with all the SEC-001 through SEC-008 features.
- [ ] Have a valid AWS credential ready to export in the terminal.