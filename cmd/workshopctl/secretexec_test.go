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

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/sdk"
)

// secretExecSuite exercises the workshopctl secret-exec subcommand, which
// redeems secret keys, arms the environment and execs the recorded binary.
type secretExecSuite struct {
	oldSecretsDir string
	oldArm        func(argv0 string, argv []string, envv []string) error

	execTarget string
	execArgs   []string
	execEnv    []string
}

var _ = check.Suite(&secretExecSuite{})

func (s *secretExecSuite) SetUpTest(c *check.C) {
	s.oldSecretsDir = dirs.WorkshopSecretsDir
	s.oldArm = armBinary

	dirs.WorkshopSecretsDir = c.MkDir()

	armBinary = func(argv0 string, argv []string, envv []string) error {
		s.execTarget = argv0
		s.execArgs = argv
		s.execEnv = envv
		return nil
	}
}

func (s *secretExecSuite) TearDownTest(c *check.C) {
	dirs.WorkshopSecretsDir = s.oldSecretsDir
	armBinary = s.oldArm
}

func (s *secretExecSuite) writeKey(c *check.C, key string, entry sdk.SecretKey) {
	data, err := entry.Marshal()
	c.Assert(err, check.IsNil)
	c.Assert(os.WriteFile(sdk.SecretKeyPath(key), data, 0644), check.IsNil)
}

// TestParseSecretExecArgs checks parsing of keys and forwarded arguments,
// split on the "--" separator.
func (s *secretExecSuite) TestParseSecretExecArgs(c *check.C) {
	parsed, err := parseSecretExecArgs([]string{"key-1", "key-2", "--", "chat", "--model", "x"})
	c.Assert(err, check.IsNil)
	c.Check(parsed.keys, check.DeepEquals, []string{"key-1", "key-2"})
	c.Check(parsed.args, check.DeepEquals, []string{"chat", "--model", "x"})
}

// TestParseSecretExecArgsNoKeys checks that an invocation without keys is a
// usage error.
func (s *secretExecSuite) TestParseSecretExecArgsNoKeys(c *check.C) {
	_, err := parseSecretExecArgs([]string{"--", "chat"})
	c.Check(err, check.ErrorMatches, "usage: workshopctl secret-exec .*")
}

// TestRunSecretExec checks that keys are redeemed, the environment is armed and
// the recorded target binary is executed verbatim with forwarded arguments.
func (s *secretExecSuite) TestRunSecretExec(c *check.C) {
	s.writeKey(c, "key-1", sdk.SecretKey{Command: "claude", Target: "/opt/claude-sdk/bin/claude", EnvMapping: "ANTHROPIC_API_KEY", Plug: "anthropic-api-key", Sdk: "claude-sdk"})
	s.writeKey(c, "key-2", sdk.SecretKey{Command: "claude", Target: "/opt/claude-sdk/bin/claude", EnvMapping: "ANTHROPIC_BASE_URL", Plug: "anthropic-base-url", Sdk: "claude-sdk"})

	err := runSecretExec([]string{"key-1", "key-2", "--", "chat"})
	c.Assert(err, check.IsNil)

	c.Check(s.execTarget, check.Equals, "/opt/claude-sdk/bin/claude")
	c.Check(s.execArgs, check.DeepEquals, []string{"claude", "chat"})
	c.Check(s.execEnv, testContains, "ANTHROPIC_API_KEY=dummy-secret-value-for-anthropic-api-key")
	c.Check(s.execEnv, testContains, "ANTHROPIC_BASE_URL=dummy-secret-value-for-anthropic-base-url")
}

// TestRunSecretExecUnknownKey checks that a key absent from the registry is a
// clear error rather than a panic.
func (s *secretExecSuite) TestRunSecretExecUnknownKey(c *check.C) {
	err := runSecretExec([]string{"absent-key"})
	c.Check(err, check.ErrorMatches, "cannot redeem secret key:.*")
}

// TestRunSecretExecMismatchedTargets checks that keys arming different binaries
// are rejected.
func (s *secretExecSuite) TestRunSecretExecMismatchedTargets(c *check.C) {
	s.writeKey(c, "key-1", sdk.SecretKey{Command: "claude", Target: "/opt/a/claude", EnvMapping: "ANTHROPIC_API_KEY", Plug: "anthropic-api-key", Sdk: "claude-sdk"})
	s.writeKey(c, "key-2", sdk.SecretKey{Command: "codex", Target: "/opt/b/codex", EnvMapping: "OPENAI_API_KEY", Plug: "openai", Sdk: "claude-sdk"})

	err := runSecretExec([]string{"key-1", "key-2"})
	c.Check(err, check.ErrorMatches, `secret keys arm different binaries .*`)
}

// testContains is a checker asserting a []string contains an exact element.
var testContains check.Checker = &containsChecker{}

type containsChecker struct{}

func (c *containsChecker) Info() *check.CheckerInfo {
	return &check.CheckerInfo{Name: "testContains", Params: []string{"obtained", "element"}}
}

func (c *containsChecker) Check(params []any, names []string) (bool, string) {
	slice, ok := params[0].([]string)
	if !ok {
		return false, "obtained value is not a []string"
	}
	want, ok := params[1].(string)
	if !ok {
		return false, "element is not a string"
	}
	for _, got := range slice {
		if got == want {
			return true, ""
		}
	}
	return false, ""
}
