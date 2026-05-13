// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3.

package builtin

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

// failConn returns a connOpener that always fails with the given message.
func failConn(msg string) connOpener {
	return func(uid string) (*dbus.Conn, error) {
		return nil, fmt.Errorf("%s", msg)
	}
}

// ConnectSessionBus is exported for integration tests.
var ConnectSessionBus = connectSessionBus
