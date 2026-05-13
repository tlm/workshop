// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3.

package builtin

import (
	"context"
	"fmt"
	"testing"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/secrets"
)

func Test(t *testing.T) {
	check.TestingT(t)
}

type RegistrySuite struct {
	savedProviders map[string]secrets.Provider
}

var _ = check.Suite(&RegistrySuite{})

// stubProvider is a minimal Provider implementation for testing.
type stubProvider struct {
	name string
}

func (p *stubProvider) Name() string { return p.name }

func (p *stubProvider) Resolve(_ context.Context, source string) (string, error) {
	if source == "" {
		return "", fmt.Errorf("secret %q not found", source)
	}
	return "value-of-" + source, nil
}

func (s *RegistrySuite) SetUpTest(c *check.C) {
	s.savedProviders = allProviders
	allProviders = nil
}

func (s *RegistrySuite) TearDownTest(c *check.C) {
	allProviders = s.savedProviders
}

func (s *RegistrySuite) TestGetMissing(c *check.C) {
	_, ok := GetProvider("no-such-provider")
	c.Assert(ok, check.Equals, false)
}

func (s *RegistrySuite) TestRegisterAndGet(c *check.C) {
	registerProvider(&stubProvider{name: "host-env"})

	got, ok := GetProvider("host-env")
	c.Assert(ok, check.Equals, true)
	c.Assert(got.Name(), check.Equals, "host-env")
}

func (s *RegistrySuite) TestRegisterDuplicatePanics(c *check.C) {
	registerProvider(&stubProvider{name: "host-env"})
	c.Assert(func() { registerProvider(&stubProvider{name: "host-env"}) },
		check.PanicMatches, `cannot register duplicate secret provider "host-env"`)
}

func (s *RegistrySuite) TestMultipleProviders(c *check.C) {
	registerProvider(&stubProvider{name: "host-env"})
	registerProvider(&stubProvider{name: "vault"})

	_, ok := GetProvider("host-env")
	c.Assert(ok, check.Equals, true)

	_, ok = GetProvider("vault")
	c.Assert(ok, check.Equals, true)

	_, ok = GetProvider("other")
	c.Assert(ok, check.Equals, false)
}

func (s *RegistrySuite) TestMockProvider(c *check.C) {
	restore := MockProvider(&stubProvider{name: "mock"})
	defer restore()

	got, ok := GetProvider("mock")
	c.Assert(ok, check.Equals, true)
	c.Assert(got.Name(), check.Equals, "mock")

	restore()
	_, ok = GetProvider("mock")
	c.Assert(ok, check.Equals, false)
}
