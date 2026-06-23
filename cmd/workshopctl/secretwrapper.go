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
	"path/filepath"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
)

// secretWrapperCommand registers a secret-arming shim at runtime, from inside an
// SDK hook, for a secret plug whose binary is not known until the hook installs
// it.
const secretWrapperCommand = "secret-wrapper"

// secretShimMode is the permission of a generated shim.
const secretShimMode = 0755

// runSecretWrapper wraps a binary with a secret arming shim at runtime.
//
// It is invoked from an SDK hook as:
//
//	workshopctl secret-wrapper <secret-name> <binary>
//
// where <binary> is an absolute path or a name resolvable on PATH. It refuses
// secrets that already declare an explicit binary (those are armed at install
// time instead).
func runSecretWrapper(argv []string) error {
	if len(argv) != 2 {
		return fmt.Errorf("usage: workshopctl %s <secret-name> <binary>", secretWrapperCommand)
	}
	secretName, binary := argv[0], argv[1]

	// The hook context cookie is workshopctl's auth key inside a hook. In the
	// real implementation it authenticates the key-mint request to workshopd;
	// here we only require its presence to prove we run from a hook.
	if os.Getenv("WORKSHOP_COOKIE") == "" {
		return fmt.Errorf("%s can only be run from an SDK hook", secretWrapperCommand)
	}

	sdkRoot := os.Getenv("SDK")
	if sdkRoot == "" {
		return fmt.Errorf("cannot determine SDK: the SDK environment variable is not set")
	}
	sdkName := filepath.Base(sdkRoot)

	arming, err := lookupSecretPlug(sdkRoot, secretName)
	if err != nil {
		return err
	}
	if arming.Binary != "" {
		return fmt.Errorf("secret %q already declares a binary; %s is not allowed", secretName, secretWrapperCommand)
	}

	target, err := resolveWrapTarget(binary)
	if err != nil {
		return err
	}
	command := filepath.Base(target)

	// TODO: mint the key via workshopd, authenticated by the hook cookie.
	key, err := sdk.MintSecretKey()
	if err != nil {
		return fmt.Errorf("cannot mint secret key: %w", err)
	}
	entry := sdk.SecretKey{
		Command:    command,
		EnvMapping: arming.EnvMapping,
		Plug:       secretName,
		Sdk:        sdkName,
		Target:     target,
	}
	if err := writeRegistryEntry(key, entry); err != nil {
		return err
	}
	return writeShimFile(command, []string{key})
}

// lookupSecretPlug finds the secret plug named secretName in the SDK metadata.
func lookupSecretPlug(sdkRoot, secretName string) (sdk.SecretArming, error) {
	data, err := os.ReadFile(filepath.Join(sdkRoot, "meta", "sdk.yaml"))
	if err != nil {
		return sdk.SecretArming{}, fmt.Errorf("cannot read SDK metadata: %w", err)
	}
	armings, err := sdk.SecretArmingsFromYAML(data)
	if err != nil {
		return sdk.SecretArming{}, fmt.Errorf("cannot read secret plugs: %w", err)
	}
	for _, arming := range armings {
		if arming.Plug == secretName {
			return arming, nil
		}
	}
	return sdk.SecretArming{}, fmt.Errorf("no secret plug %q in SDK", secretName)
}

// resolveWrapTarget resolves the binary to an absolute executable path at wrap
// time, skipping the shim directory so a re-run does not resolve to a shim.
func resolveWrapTarget(binary string) (string, error) {
	if filepath.IsAbs(binary) {
		if !osutil.IsExec(binary) {
			return "", fmt.Errorf("%q is not an executable file", binary)
		}
		return binary, nil
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" || dir == dirs.WorkshopBinDir {
			continue
		}
		candidate := filepath.Join(dir, binary)
		if osutil.IsExec(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("cannot find %q on PATH", binary)
}

// writeRegistryEntry records the context a key resolves to in the registry.
func writeRegistryEntry(key string, entry sdk.SecretKey) error {
	if err := os.MkdirAll(dirs.WorkshopSecretsDir, secretShimMode); err != nil {
		return err
	}
	data, err := entry.Marshal()
	if err != nil {
		return err
	}
	return os.WriteFile(sdk.SecretKeyPath(key), data, 0644)
}

// writeShimFile writes a shim that hands its keys to workshopctl.
func writeShimFile(command string, keys []string) error {
	if err := os.MkdirAll(dirs.WorkshopBinDir, secretShimMode); err != nil {
		return err
	}
	path := filepath.Join(dirs.WorkshopBinDir, command)
	if err := os.WriteFile(path, []byte(sdk.SecretShimContent(keys)), secretShimMode); err != nil {
		return err
	}
	return os.Chmod(path, secretShimMode)
}
