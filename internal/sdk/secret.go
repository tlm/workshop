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

package sdk

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/canonical/x-go/randutil"
	"gopkg.in/yaml.v3"

	"github.com/canonical/workshop/internal/dirs"
)

// SecretInterface is the interface name used by plugs that supply a secret
// configuration value to a binary shipped by an SDK. A secret plug names the
// environment variable to populate and, optionally, the explicit path of the
// binary it arms. When the binary path is omitted the SDK arms it at runtime
// from a hook with "workshopctl secret-wrapper".
const SecretInterface = "secret"

// SecretExecCommand is the workshopctl subcommand a shim invokes to arm a
// binary before executing it.
const SecretExecCommand = "secret-exec"

// secretKeyTokenBytes is the entropy of a minted secret key.
const secretKeyTokenBytes = 32

const (
	// secretBinaryAttr is the plug attribute giving the explicit path of the
	// binary armed by a secret plug.
	secretBinaryAttr = "binary"

	// secretEnvMappingAttr is the plug attribute naming the environment
	// variable populated with the secret value.
	secretEnvMappingAttr = "env-mapping"
)

// secretKeyToken mints a new opaque secret key. It is a variable so tests can
// make minted keys deterministic.
var secretKeyToken = func() (string, error) {
	return randutil.CryptoToken(secretKeyTokenBytes)
}

// SecretArming describes a secret plug: the environment variable to set, the
// plug supplying the value and, when known ahead of time, the explicit path of
// the binary to arm. An empty Binary means the SDK arms it at runtime via
// workshopctl secret-wrapper.
type SecretArming struct {
	Binary     string
	EnvMapping string
	Plug       string
}

// SecretKey is the context a minted secret key resolves to. For the
// proof-of-concept the registry lives as files in the workshop; eventually
// workshopd will resolve a key to this context (and the secret value) itself.
type SecretKey struct {
	// Command is the name the binary is invoked as (the shim's filename).
	Command string `json:"command"`

	// EnvMapping is the environment variable populated with the secret value.
	EnvMapping string `json:"env-mapping"`

	// Plug is the secret plug supplying the value.
	Plug string `json:"plug"`

	// Sdk is the SDK the secret plug belongs to.
	Sdk string `json:"sdk"`

	// Target is the absolute path of the binary to execute, recorded verbatim
	// when the shim is created so no path inference happens at run time.
	Target string `json:"target"`
}

// IsSecret reports whether the plug uses the secret interface.
func (plug *PlugInfo) IsSecret() bool {
	return plug.Interface == SecretInterface
}

// secretAttrs returns the binary path (optional) and env-mapping declared by a
// secret plug.
func (plug *PlugInfo) secretAttrs() (binary, envMapping string, err error) {
	if _, ok := plug.Lookup(secretBinaryAttr); ok {
		if err := plug.Attr(secretBinaryAttr, &binary); err != nil {
			return "", "", err
		}
	}
	if err := plug.Attr(secretEnvMappingAttr, &envMapping); err != nil {
		return "", "", err
	}
	return binary, envMapping, nil
}

// Marshal encodes the key context for storage in the registry.
func (k SecretKey) Marshal() ([]byte, error) {
	return json.Marshal(k)
}

// UnmarshalSecretKey decodes a registry entry.
func UnmarshalSecretKey(data []byte) (SecretKey, error) {
	var k SecretKey
	err := json.Unmarshal(data, &k)
	return k, err
}

// MintSecretKey returns a new opaque secret key token.
func MintSecretKey() (string, error) {
	return secretKeyToken()
}

// SecretKeyPath returns the registry file path for a key.
func SecretKeyPath(key string) string {
	return filepath.Join(dirs.WorkshopSecretsDir, key)
}

// SecretShimContent returns the shell script body of a shim that hands its keys
// to workshopctl to arm the binary before executing it.
func SecretShimContent(keys []string) string {
	fields := append([]string{SecretExecCommand}, keys...)
	return fmt.Sprintf("#!/bin/sh\nexec workshopctl %s -- \"$@\"\n",
		strings.Join(fields, " "))
}

// MockSecretKeyToken replaces the key minter for tests and returns a function
// restoring the original.
func MockSecretKeyToken(f func() (string, error)) (restore func()) {
	old := secretKeyToken
	secretKeyToken = f
	return func() { secretKeyToken = old }
}

// SecretArmings returns all secret plug armings in the SDK, sorted by plug name
// for deterministic ordering.
func (i *Info) SecretArmings() []SecretArming {
	var armings []SecretArming
	for _, plug := range i.Plugs {
		if !plug.IsSecret() {
			continue
		}
		binary, envMapping, err := plug.secretAttrs()
		if err != nil {
			continue
		}
		armings = append(armings, SecretArming{
			Binary:     binary,
			EnvMapping: envMapping,
			Plug:       plug.Name,
		})
	}
	sort.Slice(armings, func(a, b int) bool {
		return armings[a].Plug < armings[b].Plug
	})
	return armings
}

// SecretArmingsFromYAML reads the secret plug armings directly from raw SDK
// YAML, without running interface sanitization. workshopctl secret-wrapper uses
// it to look up a plug's env-mapping from within a hook.
func SecretArmingsFromYAML(yamlData []byte) ([]SecretArming, error) {
	var y sdkYaml
	if err := yaml.Unmarshal(yamlData, &y); err != nil {
		return nil, err
	}
	info := &Info{Plugs: make(map[string]*PlugInfo)}
	if err := setPlugsFromSdkYaml(&y, info); err != nil {
		return nil, err
	}
	return info.SecretArmings(), nil
}

// SanitizeSecretPlug checks that a secret plug declares a non-empty env-mapping
// and carries no unknown attributes. The binary path is optional. It is used
// both by SDK validation and by the secret builtin interface sanitizer.
func SanitizeSecretPlug(plug *PlugInfo) error {
	for name := range plug.Attrs {
		if name != secretBinaryAttr && name != secretEnvMappingAttr {
			return fmt.Errorf("secret plug %q has unknown attribute %q", plug.Name, name)
		}
	}
	_, envMapping, err := plug.secretAttrs()
	if err != nil {
		return fmt.Errorf("secret plug %q: %w", plug.Name, err)
	}
	if envMapping == "" {
		return fmt.Errorf("secret plug %q must declare a non-empty %q", plug.Name, secretEnvMappingAttr)
	}
	return nil
}
