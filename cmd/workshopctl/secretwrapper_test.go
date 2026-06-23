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
	"os"
	"path/filepath"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/sdk"
)

const wrapperSdkYaml = `name: runtime-sdk
base: ubuntu@24.04
plugs:
  runtime-secret:
    interface: secret
    env-mapping: RUNTIME_SECRET
  static-secret:
    interface: secret
    binary: bin/static
    env-mapping: STATIC_SECRET
`

// secretWrapperSuite exercises workshopctl secret-wrapper, the runtime arming
// path an SDK hook uses for a binary it has just installed.
type secretWrapperSuite struct {
	oldSecretsDir string
	oldBinDir     string
	oldSdksDir    string
	oldEnv        map[string]string

	sdkRoot string
}

var _ = check.Suite(&secretWrapperSuite{})

func (s *secretWrapperSuite) SetUpTest(c *check.C) {
	s.oldSecretsDir = dirs.WorkshopSecretsDir
	s.oldBinDir = dirs.WorkshopBinDir
	s.oldSdksDir = dirs.WorkshopSdksDir
	s.oldEnv = map[string]string{
		"WORKSHOP_COOKIE": os.Getenv("WORKSHOP_COOKIE"),
		"SDK":             os.Getenv("SDK"),
		"PATH":            os.Getenv("PATH"),
	}

	root := c.MkDir()
	dirs.WorkshopSecretsDir = filepath.Join(root, "secrets")
	dirs.WorkshopBinDir = filepath.Join(root, "bin")
	dirs.WorkshopSdksDir = filepath.Join(root, "sdk")

	s.sdkRoot = sdk.SdkDir("runtime-sdk")
	metaDir := filepath.Join(s.sdkRoot, "meta")
	c.Assert(os.MkdirAll(metaDir, 0755), check.IsNil)
	c.Assert(os.WriteFile(filepath.Join(metaDir, "sdk.yaml"), []byte(wrapperSdkYaml), 0644), check.IsNil)

	c.Assert(os.Setenv("WORKSHOP_COOKIE", "hook-cookie"), check.IsNil)
	c.Assert(os.Setenv("SDK", s.sdkRoot), check.IsNil)
}

func (s *secretWrapperSuite) TearDownTest(c *check.C) {
	dirs.WorkshopSecretsDir = s.oldSecretsDir
	dirs.WorkshopBinDir = s.oldBinDir
	dirs.WorkshopSdksDir = s.oldSdksDir
	for k, v := range s.oldEnv {
		if v == "" {
			_ = os.Unsetenv(k)
		} else {
			_ = os.Setenv(k, v)
		}
	}
}

// onlyRegistryEntry returns the single registry entry written, failing if there
// isn't exactly one.
func (s *secretWrapperSuite) onlyRegistryEntry(c *check.C) sdk.SecretKey {
	entries, err := os.ReadDir(dirs.WorkshopSecretsDir)
	c.Assert(err, check.IsNil)
	c.Assert(entries, check.HasLen, 1)

	data, err := os.ReadFile(filepath.Join(dirs.WorkshopSecretsDir, entries[0].Name()))
	c.Assert(err, check.IsNil)
	key, err := sdk.UnmarshalSecretKey(data)
	c.Assert(err, check.IsNil)
	return key
}

// TestRunSecretWrapper checks that wrapping an absolute binary records the
// resolved target and writes a shim that redeems the minted key.
func (s *secretWrapperSuite) TestRunSecretWrapper(c *check.C) {
	defer sdk.MockSecretKeyToken(func() (string, error) { return "key-1", nil })()

	target := filepath.Join(c.MkDir(), "claude")
	c.Assert(os.WriteFile(target, []byte("#!/bin/sh\n"), 0755), check.IsNil)

	err := runSecretWrapper([]string{"runtime-secret", target})
	c.Assert(err, check.IsNil)

	c.Check(s.onlyRegistryEntry(c), check.DeepEquals, sdk.SecretKey{
		Command:    "claude",
		EnvMapping: "RUNTIME_SECRET",
		Plug:       "runtime-secret",
		Sdk:        "runtime-sdk",
		Target:     target,
	})

	shim, err := os.ReadFile(filepath.Join(dirs.WorkshopBinDir, "claude"))
	c.Assert(err, check.IsNil)
	c.Check(string(shim), check.Equals, "#!/bin/sh\nexec workshopctl secret-exec key-1 -- \"$@\"\n")
}

// TestRunSecretWrapperResolvesPath checks that a bare binary name is resolved on
// PATH at wrap time.
func (s *secretWrapperSuite) TestRunSecretWrapperResolvesPath(c *check.C) {
	defer sdk.MockSecretKeyToken(func() (string, error) { return "key-1", nil })()

	binDir := c.MkDir()
	c.Assert(os.WriteFile(filepath.Join(binDir, "claude"), []byte("#!/bin/sh\n"), 0755), check.IsNil)
	c.Assert(os.Setenv("PATH", binDir), check.IsNil)

	err := runSecretWrapper([]string{"runtime-secret", "claude"})
	c.Assert(err, check.IsNil)
	c.Check(s.onlyRegistryEntry(c).Target, check.Equals, filepath.Join(binDir, "claude"))
}

// TestRunSecretWrapperRefusesDeclaredBinary checks that a secret already
// declaring a binary cannot be wrapped at runtime.
func (s *secretWrapperSuite) TestRunSecretWrapperRefusesDeclaredBinary(c *check.C) {
	target := filepath.Join(c.MkDir(), "static")
	c.Assert(os.WriteFile(target, []byte("#!/bin/sh\n"), 0755), check.IsNil)

	err := runSecretWrapper([]string{"static-secret", target})
	c.Check(err, check.ErrorMatches, `secret "static-secret" already declares a binary;.*`)
}

// TestRunSecretWrapperUnknownSecret checks the error for an unknown plug name.
func (s *secretWrapperSuite) TestRunSecretWrapperUnknownSecret(c *check.C) {
	target := filepath.Join(c.MkDir(), "claude")
	c.Assert(os.WriteFile(target, []byte("#!/bin/sh\n"), 0755), check.IsNil)

	err := runSecretWrapper([]string{"missing", target})
	c.Check(err, check.ErrorMatches, `no secret plug "missing" in SDK`)
}

// TestRunSecretWrapperOutsideHook checks that the command refuses to run without
// the hook context cookie.
func (s *secretWrapperSuite) TestRunSecretWrapperOutsideHook(c *check.C) {
	c.Assert(os.Unsetenv("WORKSHOP_COOKIE"), check.IsNil)

	err := runSecretWrapper([]string{"runtime-secret", "/bin/true"})
	c.Check(err, check.ErrorMatches, "secret-wrapper can only be run from an SDK hook")
}
