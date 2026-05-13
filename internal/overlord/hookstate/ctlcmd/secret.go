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

import (
	"context"
	"errors"
	"fmt"

	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/secrets"
)

var (
	shortGetSecretHelp = "Retrieve the value of a declared secret plug"
	longGetSecretHelp  = `
The get-secret command returns the resolved value of a secret plug
declared by the calling SDK in its sdkcraft.yaml.

It is intended to be invoked from SDK hooks or scripts running inside
a workshop. The command authenticates the caller via the workshopctl
context and prints the resolved secret value to stdout with no
trailing newline.
`
)

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

type getSecretPositional struct {
	PlugName string `positional-arg-name:"<plug-name>" required:"yes" description:"name of the secret plug declared in sdkcraft.yaml"`
}

type getSecretCommand struct {
	baseCommand
	getSecretPositional `positional-args:"yes"`
}

// Execute resolves the secret plug named by c.PlugName for the
// calling SDK and writes the resolved value to stdout.
func (c *getSecretCommand) Execute([]string) error {
	ctx, err := c.ensureContext()
	if err != nil {
		return err
	}

	ctx.Lock()
	defer ctx.Unlock()

	resolver, err := getSecretResolver(ctx)
	if err != nil {
		return err
	}

	plugName := c.PlugName

	value, err := resolver(context.Background(), plugName)
	if err != nil {
		if errors.Is(err, secrets.ErrUnroutedSecret) {
			return fmt.Errorf(
				"secret plug %q is not connected to a slot", plugName,
			)
		}
		return err
	}

	c.printf("%s", value)
	return nil
}

// getSecretResolver retrieves the SecretResolverFunc from the hook
// context cache.
func getSecretResolver(
	ctx *hookstate.Context,
) (secrets.SecretResolverFunc, error) {
	resolver := ctx.Cached("secret-resolver")
	if resolver == nil {
		return nil, fmt.Errorf("secret resolver not available")
	}
	r, ok := resolver.(secrets.SecretResolverFunc)
	if !ok {
		return nil, fmt.Errorf("invalid secret resolver in context")
	}
	return r, nil
}
