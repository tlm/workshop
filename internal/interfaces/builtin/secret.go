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
	"fmt"
	"slices"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/lxd_device"
	"github.com/canonical/workshop/internal/sdk"
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
var allowedSecretProviders = []string{"host-env"}

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
			return fmt.Errorf(`unknown attribute for secret interface plug: %q`, name)
		}
	}
	nameVal, ok := plug.Attrs["name"]
	if !ok {
		return fmt.Errorf(`secret plug must contain "name"`)
	}
	name, ok := nameVal.(string)
	if !ok {
		return fmt.Errorf(`"name" attribute for secret interface plug is not a string`)
	}
	if name == "" {
		return fmt.Errorf(`"name" attribute for secret interface plug must not be empty`)
	}
	return nil
}

func (iface *secretInterface) BeforePrepareSlot(slot *sdk.SlotInfo) error {
	for name := range slot.Attrs {
		if !slices.Contains(knownSecretSlotAttributes, name) {
			return fmt.Errorf(`unknown attribute for secret interface slot: %q`, name)
		}
	}

	providerVal, ok := slot.Attrs["provider"]
	if !ok {
		return fmt.Errorf(`secret slot must contain "provider"`)
	}
	provider, ok := providerVal.(string)
	if !ok {
		return fmt.Errorf(`"provider" attribute for secret interface slot is not a string`)
	}
	if !slices.Contains(allowedSecretProviders, provider) {
		return fmt.Errorf(`unsupported provider %q for secret interface slot: must be one of %v`, provider, allowedSecretProviders)
	}

	sourceVal, ok := slot.Attrs["source"]
	if !ok {
		return fmt.Errorf(`secret slot must contain "source"`)
	}
	source, ok := sourceVal.(string)
	if !ok {
		return fmt.Errorf(`"source" attribute for secret interface slot is not a string`)
	}
	if source == "" {
		return fmt.Errorf(`"source" attribute for secret interface slot must not be empty`)
	}

	return nil
}

func (iface *secretInterface) AutoConnect(plug *sdk.PlugInfo, slot *sdk.SlotInfo) bool {
	// Deny-auto-connection is enforced via the base declaration.
	return true
}

// MountConnectedPlug is a no-op placeholder until secret delivery via LXD is implemented.
func (iface *secretInterface) MountConnectedPlug(_ *lxd_device.Specification, _ *interfaces.ConnectedPlug, _ *interfaces.ConnectedSlot) error {
	return nil
}

func init() {
	registerIface(&secretInterface{})
}
