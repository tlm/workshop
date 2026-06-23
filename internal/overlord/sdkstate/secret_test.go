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

package sdkstate_test

import (
	"fmt"
	"io"
	"path/filepath"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
)

// sdkYamlSecret declares an explicit-path secret plug (armed at install) and a
// runtime secret plug without a binary (armed later via secret-wrapper).
var sdkYamlSecret = `
name: test
base: ubuntu@22.04
plugs:
  anthropic-api-key:
    interface: secret
    binary: bin/claude
    env-mapping: ANTHROPIC_API_KEY
  runtime-secret:
    interface: secret
    env-mapping: RUNTIME_SECRET
`

// mockSecretKeys makes minted keys deterministic ("key-1", "key-2", ...) for
// the duration of a test.
func mockSecretKeys() func() {
	n := 0
	return sdk.MockSecretKeyToken(func() (string, error) {
		n++
		return fmt.Sprintf("key-%d", n), nil
	})
}

// readWorkshopFile reads a file from the "ws" workshop filesystem.
func (s *sdkStateSuite) readWorkshopFile(c *check.C, path string) string {
	wfs, err := s.backend.WorkshopFs(s.ctx, "ws")
	c.Assert(err, check.IsNil)
	defer wfs.Close()

	f, err := wfs.Open(path)
	c.Assert(err, check.IsNil)
	defer f.Close()

	data, err := io.ReadAll(f)
	c.Assert(err, check.IsNil)
	return string(data)
}

func (s *sdkStateSuite) workshopFileMode(c *check.C, path string) int {
	wfs, err := s.backend.WorkshopFs(s.ctx, "ws")
	c.Assert(err, check.IsNil)
	defer wfs.Close()

	info, err := wfs.Stat(path)
	c.Assert(err, check.IsNil)
	return int(info.Mode().Perm())
}

func (s *sdkStateSuite) workshopFileAbsent(c *check.C, path string) {
	wfs, err := s.backend.WorkshopFs(s.ctx, "ws")
	c.Assert(err, check.IsNil)
	defer wfs.Close()

	_, err = wfs.Stat(path)
	c.Check(err, check.NotNil)
}

// TestDoInstallSdkArmsSecretBinaries checks that installing an SDK arms only the
// secret plug with an explicit binary: it mints a key, records the resolved
// target path, and writes a shim named after the binary. The runtime plug
// (no binary) is left for secret-wrapper.
func (s *sdkStateSuite) TestDoInstallSdkArmsSecretBinaries(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	defer sdk.MockSanitizePlugsSlots(func(sdkInfo *sdk.Info) {})()
	defer mockSecretKeys()()

	newSdk := sdk.Meta{
		Setup: sdk.Setup{
			Name:      "test",
			PackageID: "a9J51jhjzpckN8VxhqoZ8dNKcZ7pOrBb",
			Channel:   "latest/stable",
			Revision:  sdk.R(2),
			Sha3_384:  "e516dabb23b6e30026863543282780a3ae0dccf05551cf0295178d7ff0f1b41eecb9db3ff219007c4e097260d58621bd",
		},
		SdkYAML: sdkYamlSecret,
	}
	s.mockSdk(c, newSdk)

	t := s.state.NewTask("install-sdk", "test")
	t.Set("sdk", newSdk.Name)

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t)
	chg.Set("user", "testuser")
	chg.Set("ws_new_sdks", []sdk.Setup{newSdk.Setup})
	chg.AddTask(t)

	s.state.Unlock()
	c.Check(s.se.Ensure(), check.IsNil)
	s.se.Wait()
	s.state.Lock()

	c.Check(chg.Err(), check.IsNil)
	c.Check(chg.Status(), check.Equals, state.DoneStatus)

	shimPath := filepath.Join(dirs.WorkshopBinDir, "claude")
	c.Check(s.readWorkshopFile(c, shimPath), check.Equals,
		"#!/bin/sh\nexec workshopctl secret-exec key-1 -- \"$@\"\n")
	c.Check(s.workshopFileMode(c, shimPath), check.Equals, 0755)

	entry, err := sdk.UnmarshalSecretKey([]byte(s.readWorkshopFile(c, sdk.SecretKeyPath("key-1"))))
	c.Assert(err, check.IsNil)
	c.Check(entry, check.DeepEquals, sdk.SecretKey{
		Command:    "claude",
		EnvMapping: "ANTHROPIC_API_KEY",
		Plug:       "anthropic-api-key",
		Sdk:        "test",
		Target:     filepath.Join(sdk.SdkDir("test"), "bin", "claude"),
	})

	// The runtime plug (no binary) is not armed at install: no second key.
	s.workshopFileAbsent(c, sdk.SecretKeyPath("key-2"))
}

// TestUndoInstallSdkDisarmsSecretBinaries checks that undoing an install removes
// both the shim and the key registry entry.
func (s *sdkStateSuite) TestUndoInstallSdkDisarmsSecretBinaries(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	defer sdk.MockSanitizePlugsSlots(func(sdkInfo *sdk.Info) {})()
	defer mockSecretKeys()()

	newSdk := sdk.Meta{
		Setup: sdk.Setup{
			Name:      "test",
			PackageID: "a9J51jhjzpckN8VxhqoZ8dNKcZ7pOrBb",
			Channel:   "latest/stable",
			Revision:  sdk.R(1),
			Sha3_384:  "e516dabb23b6e30026863543282780a3ae0dccf05551cf0295178d7ff0f1b41eecb9db3ff219007c4e097260d58621bd",
		},
		SdkYAML: sdkYamlSecret,
	}
	s.mockSdk(c, newSdk)

	t := s.state.NewTask("install-sdk", "test")
	t.Set("sdk", newSdk.Name)

	terr := s.state.NewTask("error-trigger", "provoking total undo")
	terr.WaitFor(t)

	chg := s.state.NewChange("sample", "...")
	chg.Set("project-id", s.project.ProjectId)
	chg.Set("user", "testuser")
	chg.Set("ws_new_sdks", []sdk.Setup{newSdk.Setup})
	chg.AddTask(t)
	chg.AddTask(terr)
	setWorkshopProject("ws", s.project, t, terr)

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		c.Check(s.se.Ensure(), check.IsNil)
		s.se.Wait()
	}
	s.state.Lock()

	c.Check(t.Status(), check.Equals, state.UndoneStatus)

	s.workshopFileAbsent(c, filepath.Join(dirs.WorkshopBinDir, "claude"))
	s.workshopFileAbsent(c, sdk.SecretKeyPath("key-1"))
}
