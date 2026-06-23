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

package builtin_test

import (
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
)

// secretSuite exercises the secret interface, which arms an SDK binary with a
// secret configuration value via the environment.
type secretSuite struct {
	iface     interfaces.Interface
	projectId string
}

var _ = check.Suite(&secretSuite{
	iface: builtin.MustInterface("secret"),
})

func (s *secretSuite) SetUpSuite(c *check.C) {
	s.projectId = "42424242"
}

func (s *secretSuite) plug(c *check.C, yaml string) *sdk.PlugInfo {
	info := sdk.MockInfo(c, yaml, s.projectId, "ws")
	return info.Plugs["anthropic-api-key"]
}

func (s *secretSuite) TestName(c *check.C) {
	c.Assert(s.iface.Name(), check.Equals, "secret")
}

func (s *secretSuite) TestInterfaces(c *check.C) {
	c.Check(builtin.Interfaces(), testutil.DeepContains, s.iface)
}

// TestBeforePreparePlugValid checks that a well-formed secret plug sanitizes
// without error.
func (s *secretSuite) TestBeforePreparePlugValid(c *check.C) {
	plug := s.plug(c, `name: claude-sdk
plugs:
  anthropic-api-key:
    interface: secret
    binary: claude
    env-mapping: ANTHROPIC_API_KEY
`)
	c.Check(interfaces.BeforePreparePlug(s.iface, plug), check.IsNil)
}

// TestBeforePreparePlugMissingAttr checks that a secret plug missing a required
// attribute is rejected during sanitization.
func (s *secretSuite) TestBeforePreparePlugMissingAttr(c *check.C) {
	plug := s.plug(c, `name: claude-sdk
plugs:
  anthropic-api-key:
    interface: secret
    binary: claude
`)
	c.Check(interfaces.BeforePreparePlug(s.iface, plug), check.ErrorMatches, `.*env-mapping.*`)
}
