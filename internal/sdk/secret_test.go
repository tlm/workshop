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

package sdk_test

import (
	"path/filepath"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/sdk"
)

const secretSdkYaml = `name: claude-sdk
base: ubuntu@24.04
plugs:
  anthropic-api-key:
    interface: secret
    binary: bin/claude
    env-mapping: ANTHROPIC_API_KEY
  anthropic-base-url:
    interface: secret
    binary: bin/claude
    env-mapping: ANTHROPIC_BASE_URL
  runtime-secret:
    interface: secret
    env-mapping: RUNTIME_SECRET
`

// TestSecretPlugParsing checks that a secret plug parses and is reported as a
// secret.
func (s *SdkSuite) TestSecretPlugParsing(c *check.C) {
	info, err := sdk.ReadSdkInfo([]byte(secretSdkYaml), s.projectId, "ws")
	c.Assert(err, check.IsNil)
	c.Assert(sdk.Validate(info), check.IsNil)

	plug := info.Plugs["anthropic-api-key"]
	c.Assert(plug, check.NotNil)
	c.Check(plug.IsSecret(), check.Equals, true)
}

// TestSecretArmings checks that armings are returned sorted by plug name, with
// the explicit binary path captured and empty for runtime-registered plugs.
func (s *SdkSuite) TestSecretArmings(c *check.C) {
	info, err := sdk.ReadSdkInfo([]byte(secretSdkYaml), s.projectId, "ws")
	c.Assert(err, check.IsNil)

	c.Check(info.SecretArmings(), check.DeepEquals, []sdk.SecretArming{
		{Binary: "bin/claude", EnvMapping: "ANTHROPIC_API_KEY", Plug: "anthropic-api-key"},
		{Binary: "bin/claude", EnvMapping: "ANTHROPIC_BASE_URL", Plug: "anthropic-base-url"},
		{Binary: "", EnvMapping: "RUNTIME_SECRET", Plug: "runtime-secret"},
	})
}

// TestSecretArmingsFromYAML checks the sanitization-free reader used by
// secret-wrapper.
func (s *SdkSuite) TestSecretArmingsFromYAML(c *check.C) {
	armings, err := sdk.SecretArmingsFromYAML([]byte(secretSdkYaml))
	c.Assert(err, check.IsNil)
	c.Check(armings, check.HasLen, 3)
}

// TestSecretKeyRoundTrip checks that a key registry entry survives a
// marshal/unmarshal round trip.
func (s *SdkSuite) TestSecretKeyRoundTrip(c *check.C) {
	key := sdk.SecretKey{
		Command:    "claude",
		EnvMapping: "ANTHROPIC_API_KEY",
		Plug:       "anthropic-api-key",
		Sdk:        "claude-sdk",
		Target:     "/var/lib/workshop/sdk/claude-sdk/bin/claude",
	}
	data, err := key.Marshal()
	c.Assert(err, check.IsNil)

	got, err := sdk.UnmarshalSecretKey(data)
	c.Assert(err, check.IsNil)
	c.Check(got, check.DeepEquals, key)
}

// TestMintSecretKey checks that minting returns the mocked token and that the
// registry path is derived from it.
func (s *SdkSuite) TestMintSecretKey(c *check.C) {
	defer sdk.MockSecretKeyToken(func() (string, error) { return "deadbeef", nil })()

	key, err := sdk.MintSecretKey()
	c.Assert(err, check.IsNil)
	c.Check(key, check.Equals, "deadbeef")
	c.Check(sdk.SecretKeyPath(key), check.Equals, filepath.Join(dirs.WorkshopSecretsDir, "deadbeef"))
}

// TestSecretShimContent checks the generated shim body.
func (s *SdkSuite) TestSecretShimContent(c *check.C) {
	c.Check(sdk.SecretShimContent([]string{"key-1", "key-2"}), check.Equals,
		"#!/bin/sh\nexec workshopctl secret-exec key-1 key-2 -- \"$@\"\n")
}

// TestSanitizeSecretPlug checks that a well-formed secret plug passes
// sanitization. The suite mocks SanitizePlugsSlots to a no-op, so the plug
// survives parsing for direct inspection.
func (s *SdkSuite) TestSanitizeSecretPlug(c *check.C) {
	info, err := sdk.ReadSdkInfo([]byte(secretSdkYaml), s.projectId, "ws")
	c.Assert(err, check.IsNil)
	c.Check(sdk.SanitizeSecretPlug(info.Plugs["anthropic-api-key"]), check.IsNil)
}

// TestSanitizeSecretPlugNoBinary checks that a secret plug without a binary is
// valid (it is registered at runtime via secret-wrapper).
func (s *SdkSuite) TestSanitizeSecretPlugNoBinary(c *check.C) {
	info, err := sdk.ReadSdkInfo([]byte(secretSdkYaml), s.projectId, "ws")
	c.Assert(err, check.IsNil)
	c.Check(sdk.SanitizeSecretPlug(info.Plugs["runtime-secret"]), check.IsNil)
}

// TestSanitizeSecretPlugMissingEnvMapping checks that sanitization rejects a
// secret plug without an env-mapping attribute.
func (s *SdkSuite) TestSanitizeSecretPlugMissingEnvMapping(c *check.C) {
	info, err := sdk.ReadSdkInfo([]byte(`name: claude-sdk
plugs:
  anthropic-api-key:
    interface: secret
    binary: bin/claude
`), s.projectId, "ws")
	c.Assert(err, check.IsNil)
	c.Check(sdk.SanitizeSecretPlug(info.Plugs["anthropic-api-key"]), check.ErrorMatches, `secret plug "anthropic-api-key".*env-mapping.*`)
}

// TestSanitizeSecretPlugUnknownAttr checks that sanitization rejects a secret
// plug carrying an unexpected attribute.
func (s *SdkSuite) TestSanitizeSecretPlugUnknownAttr(c *check.C) {
	info, err := sdk.ReadSdkInfo([]byte(`name: claude-sdk
plugs:
  anthropic-api-key:
    interface: secret
    binary: bin/claude
    env-mapping: ANTHROPIC_API_KEY
    bogus: nope
`), s.projectId, "ws")
	c.Assert(err, check.IsNil)
	c.Check(sdk.SanitizeSecretPlug(info.Plugs["anthropic-api-key"]), check.ErrorMatches, `secret plug "anthropic-api-key" has unknown attribute "bogus"`)
}
