// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3.

package secrets_test

import (
	"context"
	"fmt"
	"testing"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/interfaces"
	_ "github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/secrets"
	"github.com/canonical/workshop/internal/testutil"
)

func Test(t *testing.T) {
	check.TestingT(t)
}

type resolverSuite struct {
	testutil.BaseTest
	repo      *interfaces.Repository
	resolver  *secrets.Resolver
	projectID string
}

var _ = check.Suite(&resolverSuite{})

func (s *resolverSuite) SetUpTest(c *check.C) {
	s.BaseTest.SetUpTest(c)
	s.projectID = "42424242"

	s.repo = interfaces.NewRepository()
	iface, err := interfaces.ByName("secret")
	c.Assert(err, check.IsNil)
	c.Assert(s.repo.AddInterface(iface), check.IsNil)
}

// mockProvider is a minimal Provider implementation for testing.
type mockProvider struct {
	name string
}

func (p *mockProvider) Name() string { return p.name }

func (p *mockProvider) Resolve(
	_ context.Context, source string,
) (string, error) {
	if source == "" {
		return "", fmt.Errorf("empty source")
	}
	return "resolved-" + source, nil
}

func (s *resolverSuite) mockLookup() secrets.ProviderLookup {
	providers := map[string]secrets.Provider{
		"mock": &mockProvider{name: "mock"},
	}
	return func(name string) (secrets.Provider, bool) {
		p, ok := providers[name]
		return p, ok
	}
}

func (s *resolverSuite) mockPlug(c *check.C, name string) *sdk.PlugInfo {
	yaml := fmt.Sprintf(`
name: consumer
base: ubuntu@22.04
plugs:
  %s:
    interface: secret
    name: AWS_CREDS
`, name)
	info := sdk.MockInfo(c, yaml, s.projectID, "ws")
	return info.Plugs[name]
}

func (s *resolverSuite) mockSlot(
	c *check.C, name, provider, source string,
) *sdk.SlotInfo {
	yaml := fmt.Sprintf(`
name: system
base: ubuntu@22.04
type: system
slots:
  %s:
    interface: secret
    provider: %s
    source: %s
`, name, provider, source)
	info := sdk.MockInfo(c, yaml, s.projectID, "ws")
	return info.Slots[name]
}

func (s *resolverSuite) TestResolvePlugConnected(c *check.C) {
	s.resolver = secrets.NewResolver(s.repo, s.mockLookup())

	consumer := s.mockPlug(c, "aws-credentials")
	c.Assert(s.repo.AddPlug(consumer), check.IsNil)

	provider := s.mockSlot(c, "aws-creds-provider", "mock", "aws-key")
	c.Assert(s.repo.AddSlot(provider), check.IsNil)

	allowConnect := func(
		*interfaces.ConnectedPlug,
		*interfaces.ConnectedSlot,
	) (bool, error) {
		return true, nil
	}
	_, err := s.repo.Connect(
		interfaces.NewConnRef(consumer, provider),
		nil, nil, nil, nil, allowConnect,
	)
	c.Assert(err, check.IsNil)

	value, err := s.resolver.ResolvePlug(
		context.Background(),
		s.projectID, "ws", "consumer", "aws-credentials",
	)
	c.Assert(err, check.IsNil)
	c.Assert(value, check.Equals, "resolved-aws-key")
}

func (s *resolverSuite) TestResolvePlugUnconnected(c *check.C) {
	s.resolver = secrets.NewResolver(s.repo, s.mockLookup())

	plug := s.mockPlug(c, "aws-credentials")
	c.Assert(s.repo.AddPlug(plug), check.IsNil)

	_, err := s.resolver.ResolvePlug(
		context.Background(),
		s.projectID, "ws", "consumer", "aws-credentials",
	)
	c.Assert(err, check.ErrorMatches,
		`secret plug "aws-credentials" is not connected to a `+
			`slot: secret plug is not connected to a slot`,
	)
}

func (s *resolverSuite) TestResolvePlugMissingProvider(c *check.C) {
	s.resolver = secrets.NewResolver(s.repo, s.mockLookup())

	consumer := s.mockPlug(c, "aws-credentials")
	c.Assert(s.repo.AddPlug(consumer), check.IsNil)

	provider := s.mockSlot(
		c, "aws-creds-provider", "missing-provider", "aws-key",
	)
	c.Assert(s.repo.AddSlot(provider), check.IsNil)

	allowConnect := func(
		*interfaces.ConnectedPlug,
		*interfaces.ConnectedSlot,
	) (bool, error) {
		return true, nil
	}
	_, err := s.repo.Connect(
		interfaces.NewConnRef(consumer, provider),
		nil, nil, nil, nil, allowConnect,
	)
	c.Assert(err, check.IsNil)

	_, err = s.resolver.ResolvePlug(
		context.Background(),
		s.projectID, "ws", "consumer", "aws-credentials",
	)
	c.Assert(err, check.ErrorMatches,
		`secret provider "missing-provider" for plug `+
			`"aws-credentials" is not registered`,
	)
}

func (s *resolverSuite) TestResolvePlugNotFound(c *check.C) {
	s.resolver = secrets.NewResolver(s.repo, s.mockLookup())

	_, err := s.resolver.ResolvePlug(
		context.Background(),
		s.projectID, "ws", "consumer", "no-such-plug",
	)
	c.Assert(err, check.ErrorMatches,
		`secret plug "no-such-plug" not found for .*`,
	)
}
