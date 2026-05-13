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

package builtin

import (
	"context"
	"fmt"
	"slices"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/lxd_device"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/secrets"
	secretsbuiltin "github.com/canonical/workshop/internal/secrets/builtin"
	"github.com/canonical/workshop/internal/workshop"
)

const secretSummary = `allows SDKs to declare and consume secrets from the Workshop environment`

const secretBaseDeclarationSlots = `
  secret:
    allow-installation:
      slot-sdk-type:
        - system
    allow-connection: true
    deny-auto-connection: true
`

const secretBaseDeclarationPlugs = `
  secret:
    allow-installation:
      plug-sdk-type:
        - regular
    allow-connection: true
    deny-auto-connection: true
`

var knownSecretPlugAttributes = []string{"name"}
var knownSecretSlotAttributes = []string{"provider", "source"}

// TODO: replace with registry
var allowedSecretProviders = []string{"secret-service"}

type secretInterface struct{}

func (iface *secretInterface) Name() string {
	return "secret"
}

func (iface *secretInterface) StaticInfo() interfaces.StaticInfo {
	return interfaces.StaticInfo{
		Summary:              secretSummary,
		BaseDeclarationPlugs: secretBaseDeclarationPlugs,
		BaseDeclarationSlots: secretBaseDeclarationSlots,
	}
}

func (iface *secretInterface) BeforePreparePlug(plug *sdk.PlugInfo) error {
	for name := range plug.Attrs {
		if !slices.Contains(knownSecretPlugAttributes, name) {
			return fmt.Errorf(
				`unknown attribute for secret interface plug: %q`,
				name,
			)
		}
	}
	nameVal, ok := plug.Attrs["name"]
	if !ok {
		return fmt.Errorf(`secret plug must contain "name"`)
	}
	name, ok := nameVal.(string)
	if !ok {
		return fmt.Errorf(
			`"name" attribute for secret interface plug is not a string`,
		)
	}
	if name == "" {
		return fmt.Errorf(
			`"name" attribute for secret interface plug must not be empty`,
		)
	}
	return nil
}

func (iface *secretInterface) BeforePrepareSlot(slot *sdk.SlotInfo) error {
	for name := range slot.Attrs {
		if !slices.Contains(knownSecretSlotAttributes, name) {
			return fmt.Errorf(
				`unknown attribute for secret interface slot: %q`, name,
			)
		}
	}

	providerVal, ok := slot.Attrs["provider"]
	if !ok {
		return fmt.Errorf(`secret slot must contain "provider"`)
	}
	provider, ok := providerVal.(string)
	if !ok {
		return fmt.Errorf(
			`"provider" attribute for secret interface slot is not a string`,
		)
	}
	if !slices.Contains(allowedSecretProviders, provider) {
		return fmt.Errorf(
			`unsupported provider %q for secret interface slot: `+
				`must be one of %v`,
			provider, allowedSecretProviders,
		)
	}

	sourceVal, ok := slot.Attrs["source"]
	if !ok {
		return fmt.Errorf(`secret slot must contain "source"`)
	}
	source, ok := sourceVal.(string)
	if !ok {
		return fmt.Errorf(
			`"source" attribute for secret interface slot is not a string`,
		)
	}
	if source == "" {
		return fmt.Errorf(
			`"source" attribute for secret interface slot must not be empty`,
		)
	}

	return nil
}

func (iface *secretInterface) AutoConnect(
	plug *sdk.PlugInfo, slot *sdk.SlotInfo,
) bool {
	// Deny-auto-connection is enforced via the base declaration.
	return true
}

// MountConnectedPlug verifies that the provider named by the connected
// slot is registered in the secrets registry and that the configured
// source resolves successfully. Both checks must pass for the connection
// to be accepted; otherwise the connection is rejected so that it can be
// retried once the provider is available or the source is corrected.
func (iface *secretInterface) MountConnectedPlug(
	spec *lxd_device.Specification,
	_ *interfaces.ConnectedPlug,
	slot *interfaces.ConnectedSlot,
) error {
	var providerName string
	if err := slot.Attr("provider", &providerName); err != nil {
		return fmt.Errorf(
			"cannot read provider attribute from secret slot: %w", err,
		)
	}

	var source string
	if err := slot.Attr("source", &source); err != nil {
		return fmt.Errorf(
			"cannot read source attribute from secret slot: %w", err,
		)
	}

	// Compile-time assertion that GetSecretProvider matches the lookup
	// signature used by the Resolver.
	var _ secrets.ProviderLookup = GetSecretProvider

	prov, ok := GetSecretProvider(providerName)
	if !ok {
		return fmt.Errorf(
			"secret provider %q is not registered", providerName,
		)
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, workshop.ContextUser, spec.User.Username)
	if _, err := prov.Resolve(ctx, source); err != nil {
		return fmt.Errorf(
			"secret provider %q cannot resolve source %q: %w",
			providerName, source, err,
		)
	}

	return nil
}

// GetSecretProvider looks up a secret provider by name. It is a thin
// wrapper around the builtin registry that can be overridden in tests.
var GetSecretProvider = func(name string) (secrets.Provider, bool) {
	return secretsbuiltin.GetProvider(name)
}

func init() {
	registerIface(&secretInterface{})
}
