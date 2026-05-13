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

package ctlcmd

var (
	shortGetSecretHelp = "Retrieve the value of a declared secret plug"
	longGetSecretHelp  = `
The get-secret command returns the resolved value of a secret plug
declared by the calling SDK in its sdkcraft.yaml.

It is intended to be invoked from SDK hooks or scripts running inside
a workshop. The command authenticates the caller via the workshopctl
context and prints the resolved secret value to stdout with no
trailing newline.

Resolution against a configured route and provider is not yet
implemented; for now the command returns a stub value so SDKs and
tooling can be wired end to end.
`
)

// init registers get-secret with the workshopctl command set so it is
// discoverable by the top-level parser in ctlcmd.go.
//
// Note: get-secret must also appear in nonRootAllowed for SDK hooks
// running as a non-root user to invoke it without sudo.
func init() {
	generator := func() command {
		return &getSecretCommand{}
	}
	addCommand(
		"get-secret",
		shortGetSecretHelp,
		longGetSecretHelp,
		generator,
	)
}

// getSecretPositional holds the positional arguments accepted by the
// get-secret subcommand.
//
// Fields are populated by go-flags during argument parsing. PlugName
// is marked required, so by the time Execute runs the parser has
// already rejected invocations that omit it.
type getSecretPositional struct {
	PlugName string `positional-arg-name:"<plug-name>" required:"yes" description:"name of the secret plug declared in sdkcraft.yaml"`
}

// getSecretCommand implements the workshopctl get-secret subcommand.
//
// It embeds baseCommand to inherit the standard stdout/stderr writers
// and the hookstate.Context plumbing shared by all workshopctl
// subcommands, and embeds getSecretPositional to expose the
// <plug-name> argument to go-flags.
type getSecretCommand struct {
	baseCommand
	getSecretPositional `positional-args:"yes"`
}

// Execute resolves the secret plug named by c.PlugName for the
// calling SDK and writes the resolved value to stdout.
//
// Preconditions:
//   - c.PlugName has been populated by go-flags (enforced via the
//     required:"yes" tag on getSecretPositional).
//   - c.c (the hookstate context) has been set by the workshopctl
//     dispatcher. This carries the authenticated SDK identity that
//     the daemon associates with the request.
//
// Postconditions:
//   - On success, only the resolved secret value is written to
//     stdout, with no trailing newline, and nil is returned. The
//     caller is responsible for consuming the value (e.g. by
//     exporting it as an environment variable for the duration of a
//     script) without persisting it.
//   - On failure, stdout is left empty, an error is returned, and
//     the workshopctl entry point in Run propagates a non-zero exit
//     code to the caller. The error message is intended for stderr
//     consumption by the caller and must not include the secret
//     value.
//
// Current behaviour: until the routing engine in SEC-005 lands this
// returns the stub string "stub-secret-value-for-<plug-name>" so the
// IPC plumbing between SDKs and the daemon can be exercised end to
// end. Plug declaration validation and provider resolution will
// replace the stub in later patches.
func (c *getSecretCommand) Execute([]string) error {
	_, err := c.ensureContext()
	if err != nil {
		return err
	}

	stubValue := "stub-secret-value-for-" + c.PlugName
	c.printf("%s", stubValue)
	return nil
}
