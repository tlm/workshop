// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3.

package builtin

import (
	"context"

	"gopkg.in/check.v1"
)

type SecretServiceSuite struct{}

var _ = check.Suite(&SecretServiceSuite{})

func (s *SecretServiceSuite) TestName(c *check.C) {
	p := &secretServiceProvider{}
	c.Assert(p.Name(), check.Equals, "secret-service")
}

func (s *SecretServiceSuite) TestRegistered(c *check.C) {
	p, ok := GetProvider("secret-service")
	c.Assert(ok, check.Equals, true)
	c.Assert(p.Name(), check.Equals, "secret-service")
}

func (s *SecretServiceSuite) TestResolveNoSessionBus(c *check.C) {
	p := &secretServiceProvider{
		openConn: failConn("no session bus"),
	}
	_, err := p.Resolve(context.Background(), "my-key")
	c.Assert(err, check.ErrorMatches, `cannot connect to session bus: no session bus`)
}
