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

package sdkstate

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/fsutil"
	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/sdk"
)

const (
	// secretShimMode is the permission of a generated shim.
	secretShimMode = 0755

	// secretRegistryMode is the permission of a key registry entry.
	secretRegistryMode = 0644
)

// armSecretBinaries arms every secret plug that declares an explicit binary
// path. For each such plug it mints a key, records the context the key resolves
// to (including the exact binary path), and writes a workshopctl shim that
// shadows the binary on PATH. Plugs without a declared binary are armed at
// runtime by the SDK via "workshopctl secret-wrapper".
func (m *SdkManager) armSecretBinaries(ctx context.Context, w string, info *sdk.Info) error {
	groups := groupArmingsByTarget(info)
	if len(groups) == 0 {
		return nil
	}

	fs, err := m.backend.WorkshopFs(ctx, w)
	if err != nil {
		return err
	}
	defer fs.Close()

	if err := fs.MkdirAll(dirs.WorkshopBinDir, secretShimMode); err != nil {
		return fmt.Errorf("cannot create shim directory: %w", err)
	}
	if err := fs.MkdirAll(dirs.WorkshopSecretsDir, secretShimMode); err != nil {
		return fmt.Errorf("cannot create secret registry directory: %w", err)
	}

	for _, group := range groups {
		command := filepath.Base(group.target)
		keys := make([]string, 0, len(group.armings))
		for _, arming := range group.armings {
			key, err := sdk.MintSecretKey()
			if err != nil {
				return fmt.Errorf("cannot mint secret key for %q: %w", arming.Plug, err)
			}
			entry := sdk.SecretKey{
				Command:    command,
				EnvMapping: arming.EnvMapping,
				Plug:       arming.Plug,
				Sdk:        info.Name,
				Target:     group.target,
			}
			if err := writeSecretKey(fs, key, entry); err != nil {
				return fmt.Errorf("cannot register secret key for %q: %w", arming.Plug, err)
			}
			keys = append(keys, key)
		}
		if err := writeSecretShim(fs, command, keys); err != nil {
			return fmt.Errorf("cannot write secret shim for %q: %w", command, err)
		}
	}
	return nil
}

// targetGroup is the set of armings sharing a single resolved binary path.
type targetGroup struct {
	armings []sdk.SecretArming
	target  string
}

// groupArmingsByTarget groups the SDK's explicit-path secret armings by their
// resolved absolute binary path, preserving plug order.
func groupArmingsByTarget(info *sdk.Info) []targetGroup {
	index := make(map[string]int)
	var groups []targetGroup
	for _, arming := range info.SecretArmings() {
		if arming.Binary == "" {
			continue
		}
		target := resolveSecretTarget(info.Name, arming.Binary)
		i, ok := index[target]
		if !ok {
			index[target] = len(groups)
			groups = append(groups, targetGroup{target: target})
			i = index[target]
		}
		groups[i].armings = append(groups[i].armings, arming)
	}
	return groups
}

// resolveSecretTarget resolves a declared binary path to an absolute path. A
// relative path is taken relative to the SDK root.
func resolveSecretTarget(sdkName, binary string) string {
	if filepath.IsAbs(binary) {
		return binary
	}
	return filepath.Join(sdk.SdkDir(sdkName), binary)
}

// disarmSecretBinaries removes the SDK's shims and key registry entries. It is
// best-effort: stale artefacts must not block an uninstall, so problems are
// only logged.
func (m *SdkManager) disarmSecretBinaries(ctx context.Context, w, sk string) {
	fs, err := m.backend.WorkshopFs(ctx, w)
	if err != nil {
		logger.Noticef("cannot disarm secret binaries of %q SDK: %v", sk, err)
		return
	}
	defer fs.Close()

	entries, err := fs.ReadDir(dirs.WorkshopSecretsDir)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Noticef("cannot list secret registry of %q SDK: %v", sk, err)
		}
		return
	}

	commands := make(map[string]bool)
	for _, entry := range entries {
		path := filepath.Join(dirs.WorkshopSecretsDir, entry.Name())
		key, err := readSecretKey(fs, path)
		if err != nil {
			logger.Noticef("cannot read secret registry entry %q: %v", path, err)
			continue
		}
		if key.Sdk != sk {
			continue
		}
		commands[key.Command] = true
		if err := fs.Remove(path); err != nil && !os.IsNotExist(err) {
			logger.Noticef("cannot remove secret registry entry %q: %v", path, err)
		}
	}

	for command := range commands {
		path := filepath.Join(dirs.WorkshopBinDir, command)
		if err := fs.Remove(path); err != nil && !os.IsNotExist(err) {
			logger.Noticef("cannot remove secret shim %q: %v", path, err)
		}
	}
}

// readSecretKey reads and decodes a registry entry.
func readSecretKey(fs fsutil.Fs, path string) (sdk.SecretKey, error) {
	f, err := fs.Open(path)
	if err != nil {
		return sdk.SecretKey{}, err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return sdk.SecretKey{}, err
	}
	return sdk.UnmarshalSecretKey(data)
}

// writeSecretKey records the context a key resolves to in the registry.
func writeSecretKey(fs fsutil.Fs, key string, entry sdk.SecretKey) error {
	data, err := entry.Marshal()
	if err != nil {
		return err
	}
	return writeWorkshopFile(fs, sdk.SecretKeyPath(key), data, secretRegistryMode)
}

// writeSecretShim writes a single shim for command into the workshop bin dir.
func writeSecretShim(fs fsutil.Fs, command string, keys []string) error {
	path := filepath.Join(dirs.WorkshopBinDir, command)
	return writeWorkshopFile(fs, path, []byte(sdk.SecretShimContent(keys)), secretShimMode)
}

// writeWorkshopFile writes data to a file in the workshop fs with the given
// mode, truncating any existing content.
func writeWorkshopFile(fs fsutil.Fs, path string, data []byte, mode os.FileMode) error {
	f, err := fs.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Chmod(mode)
}
