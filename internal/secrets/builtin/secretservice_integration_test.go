// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3.

//go:build integration

package builtin_test

import (
	"context"

	"github.com/godbus/dbus/v5"
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/secrets/builtin"
)

type SecretServiceIntegrationSuite struct {
	conn        *dbus.Conn
	sessionPath dbus.ObjectPath
}

var _ = check.Suite(&SecretServiceIntegrationSuite{})

func (s *SecretServiceIntegrationSuite) SetUpSuite(c *check.C) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		c.Skip("session bus not available: " + err.Error())
	}

	// Verify the Secret Service is reachable.
	svc := conn.Object("org.freedesktop.secrets", "/org/freedesktop/secrets")
	var discard dbus.Variant
	var sessionPath dbus.ObjectPath
	err = svc.Call("org.freedesktop.Secret.Service.OpenSession", 0,
		"plain", dbus.MakeVariant("")).Store(&discard, &sessionPath)
	if err != nil {
		conn.Close()
		c.Skip("secret service not available: " + err.Error())
	}

	s.conn = conn
	s.sessionPath = sessionPath
}

func (s *SecretServiceIntegrationSuite) TearDownSuite(c *check.C) {
	if s.conn != nil {
		s.conn.Object("org.freedesktop.secrets", s.sessionPath).Call(
			"org.freedesktop.Secret.Session.Close", 0)
		s.conn.Close()
	}
}

// createSessionItem creates a secret item in the always-unlocked "session"
// collection and returns its object path for cleanup.
func (s *SecretServiceIntegrationSuite) createSessionItem(c *check.C, source, value string) dbus.ObjectPath {
	col := s.conn.Object("org.freedesktop.secrets",
		"/org/freedesktop/secrets/collection/session")

	props := map[string]dbus.Variant{
		"org.freedesktop.Secret.Item.Label": dbus.MakeVariant("workshop-integration-test"),
		"org.freedesktop.Secret.Item.Attributes": dbus.MakeVariant(map[string]string{
			"service": "workshop",
			"name":    source,
		}),
	}

	// Secret struct: (session, params, value, content-type)
	type dbusSecret struct {
		Session     dbus.ObjectPath
		Parameters  []byte
		Value       []byte
		ContentType string
	}
	secret := dbusSecret{
		Session:     s.sessionPath,
		Parameters:  []byte{},
		Value:       []byte(value),
		ContentType: "text/plain",
	}

	var itemPath dbus.ObjectPath
	var promptPath dbus.ObjectPath
	err := col.Call("org.freedesktop.Secret.Collection.CreateItem", 0,
		props, secret, true).Store(&itemPath, &promptPath)
	c.Assert(err, check.IsNil)
	c.Assert(string(itemPath), check.Not(check.Equals), "/")

	return itemPath
}

func (s *SecretServiceIntegrationSuite) deleteItem(c *check.C, path dbus.ObjectPath) {
	item := s.conn.Object("org.freedesktop.secrets", path)
	var promptPath dbus.ObjectPath
	err := item.Call("org.freedesktop.Secret.Item.Delete", 0).Store(&promptPath)
	c.Assert(err, check.IsNil)
}

func (s *SecretServiceIntegrationSuite) TestResolveExistingSecret(c *check.C) {
	itemPath := s.createSessionItem(c, "integration-key", "s3cr3t-value")
	defer s.deleteItem(c, itemPath)

	p, ok := builtin.GetProvider("secret-service")
	c.Assert(ok, check.Equals, true)

	val, err := p.Resolve(context.Background(), "integration-key")
	c.Assert(err, check.IsNil)
	c.Assert(val, check.Equals, "s3cr3t-value")
}

func (s *SecretServiceIntegrationSuite) TestResolveNotFound(c *check.C) {
	p, ok := builtin.GetProvider("secret-service")
	c.Assert(ok, check.Equals, true)

	_, err := p.Resolve(context.Background(), "no-such-secret-key-ever")
	c.Assert(err, check.ErrorMatches, `secret "no-such-secret-key-ever" not found in secret service`)
}
