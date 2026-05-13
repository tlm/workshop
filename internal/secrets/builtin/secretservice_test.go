// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3.

package builtin

import (
	"context"
	"os/user"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/workshop"
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

func (s *SecretServiceSuite) TestResolveNoUser(c *check.C) {
	p := &secretServiceProvider{}
	_, err := p.Resolve(context.Background(), "my-key")
	c.Assert(err, check.ErrorMatches, `context key user not found`)
}

func (s *SecretServiceSuite) TestResolveUserLookupFails(c *check.C) {
	restore := osutil.FakeUserLookup(func(name string) (*user.User, error) {
		return nil, user.UnknownUserError(name)
	})
	defer restore()

	ctx := context.WithValue(context.Background(), workshop.ContextUser, "ghost")
	p := &secretServiceProvider{}
	_, err := p.Resolve(ctx, "my-key")
	c.Assert(err, check.ErrorMatches, `cannot look up user "ghost": .*`)
}

func (s *SecretServiceSuite) TestResolveNoSessionBus(c *check.C) {
	restore := osutil.FakeUserLookup(func(name string) (*user.User, error) {
		return &user.User{Uid: "1000", Username: name}, nil
	})
	defer restore()

	ctx := context.WithValue(context.Background(), workshop.ContextUser, "testuser")
	p := &secretServiceProvider{
		openConn: failConn("no session bus"),
	}
	_, err := p.Resolve(ctx, "my-key")
	c.Assert(err, check.ErrorMatches, `cannot connect to session bus for user "testuser": no session bus`)
}
