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
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/overlord/hookstate/ctlcmd"
	"github.com/canonical/workshop/internal/overlord/hookstate/hooktest"
	"github.com/canonical/workshop/internal/overlord/state"
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

// TestGetSecretReturnsStub exercises the success path: a valid
// hook context plus a plug name should yield the stub value on
// stdout, nothing on stderr, and no error. The stub format is
// "stub-secret-value-for-<plug-name>" and SEC-005 will replace it
// with a real resolver call.
func (s *secretSuite) TestGetSecretReturnsStub(c *check.C) {
	args := []string{"get-secret", "aws-credentials"}
	stdout, stderr, err := ctlcmd.Run(s.mockContext, args, 0)

	c.Assert(err, check.IsNil)
	c.Check(string(stdout), check.Equals, "stub-secret-value-for-aws-credentials")
	c.Check(string(stderr), check.Equals, "")
}

// TestGetSecretNoTrailingNewline guards the contract that the
// secret value is written verbatim, with no trailing newline. SDK
// hooks typically capture the value via `$(workshopctl get-secret
// X)`, where a stray newline would corrupt downstream consumers.
func (s *secretSuite) TestGetSecretNoTrailingNewline(c *check.C) {
	args := []string{"get-secret", "aws-credentials"}
	stdout, _, err := ctlcmd.Run(s.mockContext, args, 0)

	c.Assert(err, check.IsNil)
	last := stdout[len(stdout)-1]
	c.Check(last, check.Not(check.Equals), byte('\n'))
}

// TestGetSecretMissingContext exercises the authentication
// boundary: invocations without a hook context (i.e. callers that
// did not present a valid WORKSHOP_COOKIE) must be rejected before
// any value is produced.
func (s *secretSuite) TestGetSecretMissingContext(c *check.C) {
	args := []string{"get-secret", "aws-credentials"}
	stdout, _, err := ctlcmd.Run(nil, args, 0)

	expected := `cannot invoke workshopctl operation commands ` +
		`\(here "get-secret"\) from outside of a workshop`
	c.Check(err, check.ErrorMatches, expected)
	c.Check(string(stdout), check.Equals, "")
}

// TestGetSecretRequiresPlugName exercises the required positional
// argument: go-flags must reject the invocation before Execute
// runs, so no stub value can leak when the caller forgets the
// plug name.
func (s *secretSuite) TestGetSecretRequiresPlugName(c *check.C) {
	args := []string{"get-secret"}
	stdout, _, err := ctlcmd.Run(s.mockContext, args, 0)

	expected := "the required argument `<plug-name>` was not provided"
	c.Check(err, check.ErrorMatches, expected)
	c.Check(string(stdout), check.Equals, "")
}

// TestGetSecretAllowedAsNonRoot guards the nonRootAllowed entry
// for get-secret. SDK hooks run as a non-root user inside the
// workshop, so they must be able to invoke this command without
// sudo. If the entry is ever removed from nonRootAllowed in
// ctlcmd.go this test fails with a ForbiddenCommandError.
func (s *secretSuite) TestGetSecretAllowedAsNonRoot(c *check.C) {
	args := []string{"get-secret", "aws-credentials"}
	stdout, _, err := ctlcmd.Run(s.mockContext, args, 1000)

	c.Assert(err, check.IsNil)
	c.Check(string(stdout), check.Equals, "stub-secret-value-for-aws-credentials")
}
