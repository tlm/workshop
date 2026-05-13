// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2016 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package builtin_test

import (
	"context"
	"fmt"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/interfaces/lxd_device"
	"github.com/canonical/workshop/internal/secrets"
	secretsbuiltin "github.com/canonical/workshop/internal/secrets/builtin"
	"github.com/canonical/workshop/internal/testutil"
)

type secretSuite struct {
	iface     interfaces.Interface
	projectId string
}

var _ = check.Suite(&secretSuite{
	iface: builtin.MustInterface("secret"),
})

func (s *secretSuite) SetUpTest(c *check.C) {
	s.projectId = "42424242"
}

// mockSecretProvider returns the value of values[source] or an error
// if no entry exists for that source.
type mockSecretProvider struct {
	name   string
	values map[string]string
}

func (p *mockSecretProvider) Name() string { return p.name }

func (p *mockSecretProvider) Resolve(
	_ context.Context, source string,
) (string, error) {
	v, ok := p.values[source]
	if !ok {
		return "", fmt.Errorf("source %q not available", source)
	}
	return v, nil
}

// mountConnectedPlugDefiner mirrors the unexported interface that
// lxd_device.Specification.AddConnectedPlug uses to dispatch into the
// backend; this lets the test call MountConnectedPlug directly without
// having to construct a full Specification (and stub out LXD lookups).
type mountConnectedPlugDefiner interface {
	MountConnectedPlug(
		spec *lxd_device.Specification,
		plug *interfaces.ConnectedPlug,
		slot *interfaces.ConnectedSlot,
	) error
}

// mountSecretPlug invokes MountConnectedPlug directly on the secret
// interface using mocked plug and slot info.
func (s *secretSuite) mountSecretPlug(
	c *check.C, plugYaml, slotYaml, plugSdk, plugName, slotSdk, slotName string,
) error {
	plug := builtin.MockPlug(c, plugYaml, s.projectId, "ws", plugSdk, plugName)
	connectedPlug := interfaces.NewConnectedPlug(plug, nil, nil)
	slot := builtin.MockSlot(c, slotYaml, s.projectId, "ws", slotSdk, slotName)
	connectedSlot := interfaces.NewConnectedSlot(slot, nil, nil)

	definer, ok := s.iface.(mountConnectedPlugDefiner)
	c.Assert(ok, check.Equals, true)
	return definer.MountConnectedPlug(nil, connectedPlug, connectedSlot)
}

const secretPlugYaml = `name: consumer
base: ubuntu@22.04
plugs:
  github-token:
    interface: secret
    name: GITHUB_TOKEN
`

const secretSlotYaml = `name: system
base: ubuntu@22.04
type: system
slots:
  github-token-provider:
    interface: secret
    provider: host-env
    source: GITHUB_TOKEN
`

func (s *secretSuite) TestName(c *check.C) {
	c.Assert(s.iface.Name(), check.Equals, "secret")
}

func (s *secretSuite) TestInterfaces(c *check.C) {
	c.Check(builtin.Interfaces(), testutil.DeepContains, s.iface)
}

func (s *secretSuite) TestAutoConnect(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
  github-token:
    interface: secret
    name: GITHUB_TOKEN
`, s.projectId, "ws", "consumer", "github-token")

	slot := builtin.MockSlot(c, `name: system
base: ubuntu@22.04
type: system
slots:
  aws-creds-provider:
    interface: secret
    provider: host-env
    source: AWS_ACCESS_KEY_ID
`, s.projectId, "ws", "system", "aws-creds-provider")

	// AutoConnect returns true to let the base declaration's deny-auto-connection enforce policy.
	c.Assert(s.iface.AutoConnect(plug, slot), check.Equals, true)
}

// Plug tests

func (s *secretSuite) TestPlugWithName(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
  github-token:
    interface: secret
    name: GITHUB_TOKEN
`, s.projectId, "ws", "consumer", "github-token")
	c.Assert(interfaces.BeforePreparePlug(s.iface, plug), check.IsNil)
}

func (s *secretSuite) TestPlugMissingName(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
  github-token:
    interface: secret
`, s.projectId, "ws", "consumer", "github-token")
	c.Assert(interfaces.BeforePreparePlug(s.iface, plug), check.ErrorMatches,
		`secret plug must contain "name"`)
}

func (s *secretSuite) TestPlugEmptyName(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
  github-token:
    interface: secret
    name: ""
`, s.projectId, "ws", "consumer", "github-token")
	c.Assert(interfaces.BeforePreparePlug(s.iface, plug), check.ErrorMatches,
		`"name" attribute for secret interface plug must not be empty`)
}

func (s *secretSuite) TestPlugUnknownAttribute(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
  github-token:
    interface: secret
    name: GITHUB_TOKEN
    unknown-attr: foo
`, s.projectId, "ws", "consumer", "github-token")
	c.Assert(interfaces.BeforePreparePlug(s.iface, plug), check.ErrorMatches,
		`unknown attribute for secret interface plug: "unknown-attr"`)
}

// Slot tests

func (s *secretSuite) TestSlotValid(c *check.C) {
	slot := builtin.MockSlot(c, `name: system
base: ubuntu@22.04
type: system
slots:
  aws-creds-provider:
    interface: secret
    provider: host-env
    source: AWS_ACCESS_KEY_ID
`, s.projectId, "ws", "system", "aws-creds-provider")
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.IsNil)
}

func (s *secretSuite) TestSlotMissingProvider(c *check.C) {
	slot := builtin.MockSlot(c, `name: system
base: ubuntu@22.04
type: system
slots:
  aws-creds-provider:
    interface: secret
    source: AWS_ACCESS_KEY_ID
`, s.projectId, "ws", "system", "aws-creds-provider")
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.ErrorMatches,
		`secret slot must contain "provider"`)
}

func (s *secretSuite) TestSlotMissingSource(c *check.C) {
	slot := builtin.MockSlot(c, `name: system
base: ubuntu@22.04
type: system
slots:
  aws-creds-provider:
    interface: secret
    provider: host-env
`, s.projectId, "ws", "system", "aws-creds-provider")
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.ErrorMatches,
		`secret slot must contain "source"`)
}

func (s *secretSuite) TestSlotUnsupportedProvider(c *check.C) {
	slot := builtin.MockSlot(c, `name: system
base: ubuntu@22.04
type: system
slots:
  aws-creds-provider:
    interface: secret
    provider: vault
    source: secret/aws/credentials
`, s.projectId, "ws", "system", "aws-creds-provider")
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.ErrorMatches,
		`unsupported provider "vault" for secret interface slot: must be one of \[host-env\]`)
}

func (s *secretSuite) TestSlotUnknownAttribute(c *check.C) {
	slot := builtin.MockSlot(c, `name: system
base: ubuntu@22.04
type: system
slots:
  aws-creds-provider:
    interface: secret
    provider: host-env
    source: AWS_ACCESS_KEY_ID
    unknown-attr: foo
`, s.projectId, "ws", "system", "aws-creds-provider")
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.ErrorMatches,
		`unknown attribute for secret interface slot: "unknown-attr"`)
}

func (s *secretSuite) TestSlotEmptySource(c *check.C) {
	slot := builtin.MockSlot(c, `name: system
base: ubuntu@22.04
type: system
slots:
  aws-creds-provider:
    interface: secret
    provider: host-env
    source: ""
`, s.projectId, "ws", "system", "aws-creds-provider")
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.ErrorMatches,
		`"source" attribute for secret interface slot must not be empty`)
}

// MountConnectedPlug tests

// TestMountConnectedPlugResolves verifies that the connection succeeds
// when the slot's provider is registered and the source resolves.
func (s *secretSuite) TestMountConnectedPlugResolves(c *check.C) {
	restore := secretsbuiltin.MockProvider(&mockSecretProvider{
		name:   "host-env",
		values: map[string]string{"GITHUB_TOKEN": "shh"},
	})
	defer restore()

	err := s.mountSecretPlug(
		c, secretPlugYaml, secretSlotYaml,
		"consumer", "github-token",
		"system", "github-token-provider",
	)
	c.Assert(err, check.IsNil)
}

// TestMountConnectedPlugProviderMissing verifies that the connection is
// rejected when the slot names a provider that is not registered.
func (s *secretSuite) TestMountConnectedPlugProviderMissing(c *check.C) {
	// Sanity check: no provider registered.
	_, ok := builtin.GetSecretProvider("host-env")
	c.Assert(ok, check.Equals, false)

	err := s.mountSecretPlug(
		c, secretPlugYaml, secretSlotYaml,
		"consumer", "github-token",
		"system", "github-token-provider",
	)
	c.Assert(err, check.ErrorMatches,
		`secret provider "host-env" is not registered`)
}

// TestMountConnectedPlugSourceUnresolved verifies that the connection
// is rejected when the slot's source cannot be resolved by the
// registered provider.
func (s *secretSuite) TestMountConnectedPlugSourceUnresolved(c *check.C) {
	restore := secretsbuiltin.MockProvider(&mockSecretProvider{
		name: "host-env",
		// no GITHUB_TOKEN entry -> Resolve fails
	})
	defer restore()

	err := s.mountSecretPlug(
		c, secretPlugYaml, secretSlotYaml,
		"consumer", "github-token",
		"system", "github-token-provider",
	)
	c.Assert(err, check.ErrorMatches,
		`secret provider "host-env" cannot resolve source "GITHUB_TOKEN":.*`)
}

// TestGetSecretProviderSignature is a compile-time check that the
// exported lookup matches the resolver's signature.
func (s *secretSuite) TestGetSecretProviderSignature(c *check.C) {
	var _ secrets.ProviderLookup = builtin.GetSecretProvider
}
