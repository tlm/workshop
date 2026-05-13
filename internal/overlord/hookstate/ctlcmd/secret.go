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
	"strings"

	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/secrets"
)

// WorkshopSecretResolverFunc resolves a secret plug for a workshop-scoped
// caller that does not have an implicit SDK context (e.g. an interactive
// workshop shell). The caller supplies both the SDK and the plug name.
type WorkshopSecretResolverFunc func(ctx context.Context, sdkName, plugName string) (string, error)

var (
	shortGetSecretHelp = "Retrieve the value of a declared secret plug"
	longGetSecretHelp  = `
The get-secret command returns the resolved value of a secret plug
declared by the calling SDK in its sdkcraft.yaml.

It is intended to be invoked from SDK hooks or scripts running inside
a workshop. The command authenticates the caller via the workshopctl
context and prints the resolved secret value to stdout with no
trailing newline.

When invoked from a workshop shell (rather than an SDK hook) the SDK
is not implicit; pass the qualified form "<sdk>:<plug>" to identify
which SDK owns the secret plug.
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
	PlugName string `positional-arg-name:"<plug-name>" required:"yes" description:"plug name, or qualified <sdk>:<plug> when called from a workshop shell"`
}

type getSecretCommand struct {
	baseCommand
	getSecretPositional `positional-args:"yes"`
}

// Execute resolves the secret plug named by c.PlugName and writes the
// resolved value to stdout. The bare plug form uses the calling SDK's
// hook context; the qualified "<sdk>:<plug>" form is required when the
// caller is a workshop shell with no implicit SDK.
func (c *getSecretCommand) Execute([]string) error {
	ctx, err := c.ensureContext()
	if err != nil {
		return err
	}

	ctx.Lock()
	defer ctx.Unlock()

	sdkName, plugName, qualified := splitQualifiedPlug(c.PlugName)

	value, err := resolveSecret(ctx, sdkName, plugName, qualified)
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

// splitQualifiedPlug splits an argument of the form "<sdk>:<plug>" into
// its parts. A bare plug name is returned with an empty sdk and
// qualified=false.
func splitQualifiedPlug(arg string) (sdkName, plugName string, qualified bool) {
	if idx := strings.Index(arg, ":"); idx >= 0 {
		return arg[:idx], arg[idx+1:], true
	}
	return "", arg, false
}

// resolveSecret picks the right resolver for the call site. SDK-bound
// hook contexts use "secret-resolver"; workshop-cookie contexts use
// "workshop-secret-resolver" and require a qualified plug name. The
// qualified form is also accepted from a hook so long as the SDK
// prefix matches the hook's own SDK.
func resolveSecret(
	ctx *hookstate.Context, sdkName, plugName string, qualified bool,
) (string, error) {
	if bound := ctx.Cached("secret-resolver"); bound != nil {
		r, ok := bound.(secrets.SecretResolverFunc)
		if !ok {
			return "", fmt.Errorf("invalid secret resolver in context")
		}
		if qualified && sdkName != ctx.Sdk() {
			return "", fmt.Errorf(
				"secret plug %q belongs to SDK %q but this hook runs as %q",
				plugName, sdkName, ctx.Sdk(),
			)
		}
		return r(context.Background(), plugName)
	}

	if unbound := ctx.Cached("workshop-secret-resolver"); unbound != nil {
		r, ok := unbound.(WorkshopSecretResolverFunc)
		if !ok {
			return "", fmt.Errorf("invalid secret resolver in context")
		}
		if !qualified {
			return "", fmt.Errorf(
				"plug name must be qualified as <sdk>:<plug> when called from a workshop shell",
			)
		}
		return r(context.Background(), sdkName, plugName)
	}

	return "", fmt.Errorf("secret resolver not available")
}
