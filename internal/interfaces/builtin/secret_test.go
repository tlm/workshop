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
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/builtin"
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
