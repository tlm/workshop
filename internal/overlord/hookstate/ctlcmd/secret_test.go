/*
 * Copyright (C) 2026 Canonical Ltd
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

package ctlcmd_test

import (
	"context"
	"fmt"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/overlord/hookstate/ctlcmd"
	"github.com/canonical/workshop/internal/overlord/hookstate/hooktest"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/secrets"
	"github.com/canonical/workshop/internal/testutil"
)

type secretSuite struct {
	testutil.BaseTest
	state       *state.State
	mockContext *hookstate.Context
	mockHandler *hooktest.MockHandler
}

var _ = check.Suite(&secretSuite{})

func (s *secretSuite) SetUpTest(c *check.C) {
	s.BaseTest.SetUpTest(c)
	dirs.SetRootDir(c.MkDir())

	s.mockHandler = hooktest.NewMockHandler()

	s.state = state.New(nil)
	s.state.Lock()
	defer s.state.Unlock()

	task := s.state.NewTask("test-task", "my test task")
	setup := &hookstate.HookSetup{
		Sdk:      "test-sdk",
		HookType: hookstate.SetupBase,
	}

	ctx, err := hookstate.NewContext(task, s.state, setup, s.mockHandler, "")
	c.Assert(err, check.IsNil)
	s.mockContext = ctx
}

// mockSecretResolver returns a SecretResolverFunc that resolves
// every plug to "resolved-<plug-name>".
func mockSecretResolver() secrets.SecretResolverFunc {
	return func(_ context.Context, plugName string) (string, error) {
		return "resolved-" + plugName, nil
	}
}

// injectResolver caches a mock resolver in the hook context.
func injectResolver(ctx *hookstate.Context) {
	ctx.Lock()
	defer ctx.Unlock()
	ctx.Cache("secret-resolver", mockSecretResolver())
}

// TestGetSecretSuccess exercises the success path: a valid hook
// context with an injected resolver plus a plug name yields the
// resolved value on stdout.
func (s *secretSuite) TestGetSecretSuccess(c *check.C) {
	injectResolver(s.mockContext)

	args := []string{"get-secret", "aws-credentials"}
	stdout, stderr, err := ctlcmd.Run(s.mockContext, args, 0)

	c.Assert(err, check.IsNil)
	c.Check(string(stdout), check.Equals, "resolved-aws-credentials")
	c.Check(string(stderr), check.Equals, "")
}

// TestGetSecretNoTrailingNewline guards the contract that the
// secret value is written verbatim, with no trailing newline.
func (s *secretSuite) TestGetSecretNoTrailingNewline(c *check.C) {
	injectResolver(s.mockContext)

	args := []string{"get-secret", "aws-credentials"}
	stdout, _, err := ctlcmd.Run(s.mockContext, args, 0)

	c.Assert(err, check.IsNil)
	last := stdout[len(stdout)-1]
	c.Check(last, check.Not(check.Equals), byte('\n'))
}

// TestGetSecretMissingContext exercises the authentication
// boundary: invocations without a hook context must be rejected.
func (s *secretSuite) TestGetSecretMissingContext(c *check.C) {
	args := []string{"get-secret", "aws-credentials"}
	stdout, _, err := ctlcmd.Run(nil, args, 0)

	expected := `cannot invoke workshopctl operation commands ` +
		`\(here "get-secret"\) from outside of a workshop`
	c.Check(err, check.ErrorMatches, expected)
	c.Check(string(stdout), check.Equals, "")
}

// TestGetSecretRequiresPlugName exercises the required positional
// argument.
func (s *secretSuite) TestGetSecretRequiresPlugName(c *check.C) {
	args := []string{"get-secret"}
	stdout, _, err := ctlcmd.Run(s.mockContext, args, 0)

	expected := "the required argument `<plug-name>` was not provided"
	c.Check(err, check.ErrorMatches, expected)
	c.Check(string(stdout), check.Equals, "")
}

// TestGetSecretAllowedAsNonRoot guards the nonRootAllowed entry
// for get-secret.
func (s *secretSuite) TestGetSecretAllowedAsNonRoot(c *check.C) {
	injectResolver(s.mockContext)

	args := []string{"get-secret", "aws-credentials"}
	stdout, _, err := ctlcmd.Run(s.mockContext, args, 1000)

	c.Assert(err, check.IsNil)
	c.Check(string(stdout), check.Equals, "resolved-aws-credentials")
}

// TestGetSecretUnrouted exercises the error path when the resolver
// returns ErrUnroutedSecret.
func (s *secretSuite) TestGetSecretUnrouted(c *check.C) {
	resolver := secrets.SecretResolverFunc(func(_ context.Context, plugName string) (string, error) {
		return "", fmt.Errorf(
			"secret plug %q is not connected: %w",
			plugName, secrets.ErrUnroutedSecret,
		)
	})
	s.mockContext.Lock()
	s.mockContext.Cache("secret-resolver", resolver)
	s.mockContext.Unlock()

	args := []string{"get-secret", "aws-credentials"}
	stdout, _, err := ctlcmd.Run(s.mockContext, args, 0)

	c.Assert(err, check.NotNil)
	c.Check(err, check.ErrorMatches,
		`secret plug "aws-credentials" is not connected to a slot`)
	c.Check(string(stdout), check.Equals, "")
}

// TestGetSecretNoResolver exercises the error path when no
// resolver has been injected into the context.
func (s *secretSuite) TestGetSecretNoResolver(c *check.C) {
	args := []string{"get-secret", "aws-credentials"}
	stdout, _, err := ctlcmd.Run(s.mockContext, args, 0)

	c.Assert(err, check.NotNil)
	c.Check(err, check.ErrorMatches, `secret resolver not available`)
	c.Check(string(stdout), check.Equals, "")
}
