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

package hookstate

import (
	"errors"
	"fmt"

	"github.com/canonical/x-go/randutil"

	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/workshop"
)

// WorkshopCookiesKey is the state key under which long-lived per-workshop
// cookies are stored.
const WorkshopCookiesKey = "workshop-cookies"

// WorkshopCookie is a long-lived cookie installed in a workshop that allows
// workshopctl callers inside the workshop to authenticate back to workshopd.
type WorkshopCookie struct {
	Project  workshop.Project `json:"project"`
	Workshop string           `json:"workshop"`
	User     string           `json:"user"`
}

// WorkshopCookies returns all registered workshop cookies keyed by cookie ID.
// State must be locked by the caller.
func WorkshopCookies(st *state.State) (map[string]WorkshopCookie, error) {
	var cookies map[string]WorkshopCookie
	err := st.Get(WorkshopCookiesKey, &cookies)
	if err != nil && !errors.Is(err, state.ErrNoState) {
		return nil, fmt.Errorf("cannot read workshop cookies: %w", err)
	}
	if cookies == nil {
		cookies = make(map[string]WorkshopCookie)
	}
	return cookies, nil
}

// AddWorkshopCookie generates a fresh cookie ID, drops any existing cookies
// registered for the same (project, workshop) pair, and persists the new
// record. The state must be locked by the caller. Returns the new cookie ID.
func AddWorkshopCookie(st *state.State, cookie WorkshopCookie) (string, error) {
	cookies, err := WorkshopCookies(st)
	if err != nil {
		return "", err
	}
	for id, existing := range cookies {
		if existing.Project.ProjectId == cookie.Project.ProjectId && existing.Workshop == cookie.Workshop {
			delete(cookies, id)
		}
	}
	id, err := randutil.CryptoToken(32)
	if err != nil {
		return "", fmt.Errorf("cannot generate workshop cookie: %w", err)
	}
	cookies[id] = cookie
	st.Set(WorkshopCookiesKey, cookies)
	return id, nil
}

// RemoveWorkshopCookies deletes any cookies tied to the given (project,
// workshop) pair. State must be locked by the caller.
func RemoveWorkshopCookies(st *state.State, projectId, w string) error {
	cookies, err := WorkshopCookies(st)
	if err != nil {
		return err
	}
	changed := false
	for id, c := range cookies {
		if c.Project.ProjectId == projectId && c.Workshop == w {
			delete(cookies, id)
			changed = true
		}
	}
	if changed {
		st.Set(WorkshopCookiesKey, cookies)
	}
	return nil
}
