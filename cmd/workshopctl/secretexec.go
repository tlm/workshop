// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/canonical/workshop/internal/sdk"
)

// secretExecCommand is the workshopctl subcommand a generated shim invokes to
// arm an SDK binary with its secrets before executing it.
const secretExecCommand = sdk.SecretExecCommand

// secretExecArgs holds the parsed arguments of a secret-exec invocation.
type secretExecArgs struct {
	// keys are the secret keys to redeem. Each resolves to the binary to run,
	// the secret plug and the environment variable to arm.
	keys []string
	args []string
}

// armBinary is the process-replacing exec, overridable in tests.
var armBinary = syscall.Exec

// parseSecretExecArgs parses "<key>... [-- args...]". Tokens before a "--"
// separator are secret keys; arguments after it are forwarded to the binary.
func parseSecretExecArgs(argv []string) (secretExecArgs, error) {
	var parsed secretExecArgs
	for i, arg := range argv {
		if arg == "--" {
			parsed.args = argv[i+1:]
			break
		}
		parsed.keys = append(parsed.keys, arg)
	}
	if len(parsed.keys) == 0 {
		return secretExecArgs{}, fmt.Errorf("usage: workshopctl %s <key>... [-- args...]", secretExecCommand)
	}
	return parsed, nil
}

// runSecretExec redeems each key for its arming context, arms the environment
// and replaces the current process with the recorded target binary.
func runSecretExec(argv []string) error {
	parsed, err := parseSecretExecArgs(argv)
	if err != nil {
		return err
	}

	entries, err := redeemKeys(parsed.keys)
	if err != nil {
		return err
	}

	target, command, err := commonTarget(entries)
	if err != nil {
		return err
	}

	env := armEnv(os.Environ(), entries)

	callArgs := append([]string{command}, parsed.args...)
	return armBinary(target, callArgs, env)
}

// redeemKeys resolves each key to its registered context. For the
// proof-of-concept the registry is read locally; eventually a key will be
// presented to workshopd, which resolves the context (and the secret value).
func redeemKeys(keys []string) ([]sdk.SecretKey, error) {
	entries := make([]sdk.SecretKey, 0, len(keys))
	for _, key := range keys {
		data, err := os.ReadFile(sdk.SecretKeyPath(key))
		if err != nil {
			return nil, fmt.Errorf("cannot redeem secret key: %w", err)
		}
		entry, err := sdk.UnmarshalSecretKey(data)
		if err != nil {
			return nil, fmt.Errorf("cannot decode secret key: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// commonTarget returns the target path and command shared by all key entries,
// erroring if they disagree (a shim only ever groups keys for one binary).
func commonTarget(entries []sdk.SecretKey) (target, command string, err error) {
	target, command = entries[0].Target, entries[0].Command
	if target == "" {
		return "", "", fmt.Errorf("secret key has no target binary")
	}
	for _, entry := range entries[1:] {
		if entry.Target != target {
			return "", "", fmt.Errorf("secret keys arm different binaries (%q and %q)", target, entry.Target)
		}
	}
	return target, command, nil
}

// armEnv returns env extended with the secret value for each key entry. The
// values are dummy placeholders for now; this is where workshopctl will later
// present the key to workshopd and receive the real secret value.
func armEnv(env []string, entries []sdk.SecretKey) []string {
	armed := make([]string, len(env), len(env)+len(entries))
	copy(armed, env)
	for _, entry := range entries {
		// TODO: replace the dummy value with the secret workshopd returns for
		// this key.
		value := fmt.Sprintf("dummy-secret-value-for-%s", entry.Plug)
		armed = append(armed, entry.EnvMapping+"="+value)
	}
	return armed
}
