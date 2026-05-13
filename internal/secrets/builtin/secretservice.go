// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3.

package builtin

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
)

const (
	secretServiceProviderName = "secret-service"

	dbusServiceName    = "org.freedesktop.secrets"
	dbusServicePath    = "/org/freedesktop/secrets"
	dbusServiceIface   = "org.freedesktop.Secret.Service"
	dbusSessionIface   = "org.freedesktop.Secret.Session"
	dbusItemIface      = "org.freedesktop.Secret.Item"
	dbusPropertiesIface = "org.freedesktop.DBus.Properties"
)

// connOpener abstracts D-Bus session bus connection for testability.
type connOpener func() (*dbus.Conn, error)

type secretServiceProvider struct {
	openConn connOpener
}

func (p *secretServiceProvider) Name() string {
	return secretServiceProviderName
}

func (p *secretServiceProvider) Resolve(ctx context.Context, source string) (string, error) {
	open := p.openConn
	if open == nil {
		open = func() (*dbus.Conn, error) {
			return dbus.ConnectSessionBus()
		}
	}
	conn, err := open()
	if err != nil {
		return "", fmt.Errorf("cannot connect to session bus: %w", err)
	}
	defer conn.Close()

	svc := conn.Object(dbusServiceName, dbusServicePath)

	// Open a plain-text session (safe — same machine, same user).
	var discard dbus.Variant
	var sessionPath dbus.ObjectPath
	err = svc.CallWithContext(ctx, dbusServiceIface+".OpenSession", 0,
		"plain", dbus.MakeVariant("")).Store(&discard, &sessionPath)
	if err != nil {
		return "", fmt.Errorf("cannot open secret service session: %w", err)
	}
	defer conn.Object(dbusServiceName, sessionPath).Call(dbusSessionIface+".Close", 0)

	// Search for items matching the Workshop convention.
	attrs := map[string]string{
		"service": "workshop",
		"name":    source,
	}
	var unlocked, locked []dbus.ObjectPath
	err = svc.CallWithContext(ctx, dbusServiceIface+".SearchItems", 0,
		attrs).Store(&unlocked, &locked)
	if err != nil {
		return "", fmt.Errorf("cannot search for secret %q: %w", source, err)
	}

	items := append(unlocked, locked...)
	if len(items) == 0 {
		return "", fmt.Errorf("secret %q not found in secret service", source)
	}

	// If the first match is locked, try a non-interactive unlock.
	itemPath := items[0]
	if len(unlocked) == 0 {
		var unlockedPaths []dbus.ObjectPath
		var promptPath dbus.ObjectPath
		err = svc.CallWithContext(ctx, dbusServiceIface+".Unlock", 0,
			[]dbus.ObjectPath{itemPath}).Store(&unlockedPaths, &promptPath)
		if err != nil {
			return "", fmt.Errorf("cannot unlock secret %q: %w", source, err)
		}
		if len(unlockedPaths) == 0 {
			return "", fmt.Errorf("secret %q is locked and requires interactive unlock", source)
		}
	}

	// Retrieve the secret value.
	// Secret struct on the wire: (session ObjectPath, params []byte, value []byte, content-type string).
	type dbusSecret struct {
		Session     dbus.ObjectPath
		Parameters  []byte
		Value       []byte
		ContentType string
	}
	var secrets map[dbus.ObjectPath]dbusSecret
	err = svc.CallWithContext(ctx, dbusServiceIface+".GetSecrets", 0,
		[]dbus.ObjectPath{itemPath}, sessionPath).Store(&secrets)
	if err != nil {
		return "", fmt.Errorf("cannot retrieve secret %q: %w", source, err)
	}

	s, ok := secrets[itemPath]
	if !ok {
		return "", fmt.Errorf("secret %q missing from service response", source)
	}

	return string(s.Value), nil
}

func init() {
	registerProvider(&secretServiceProvider{})
}
