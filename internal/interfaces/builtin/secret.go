// -*- Mode: Go; indent-tabs-mode: t -*-

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

package builtin

import (
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/lxd_device"
	"github.com/canonical/workshop/internal/sdk"
)

const secretSummary = `allows supplying a secret configuration value to an SDK binary via the environment`

// The slot side is a placeholder for a future secret provider (e.g. the system
// SDK). For now secret plugs are armed by workshopctl shims generated at SDK
// install time, independently of any connection.
const secretBaseDeclarationSlots = `
  secret:
    allow-installation:
      slot-sdk-type:
        - system
      slot-names:
        - $INTERFACE
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

type secretInterface struct{}

func (iface *secretInterface) Name() string {
	return sdk.SecretInterface
}

func (iface *secretInterface) StaticInfo() interfaces.StaticInfo {
	return interfaces.StaticInfo{
		Summary:              secretSummary,
		BaseDeclarationPlugs: secretBaseDeclarationPlugs,
		BaseDeclarationSlots: secretBaseDeclarationSlots,
	}
}

func (iface *secretInterface) AutoConnect(plug *sdk.PlugInfo, slot *sdk.SlotInfo) bool {
	// allow what declarations allowed
	return true
}

func (iface *secretInterface) BeforePreparePlug(plug *sdk.PlugInfo) error {
	return sdk.SanitizeSecretPlug(plug)
}

// MountPermanentPlug contributes nothing to the LXD device specification: a
// secret plug arms its binary through a workshopctl shim, not through a mounted
// device. The method exists so the secret interface satisfies the backend
// specification contract shared by all interfaces.
func (iface *secretInterface) MountPermanentPlug(spec *lxd_device.Specification, plug *sdk.PlugInfo) error {
	return nil
}

func init() {
	registerIface(&secretInterface{})
}
