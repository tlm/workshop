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

package hookstate_test

import (
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/workshop"
)

type cookiesSuite struct{}

var _ = check.Suite(&cookiesSuite{})

func (s *cookiesSuite) TestAddAndReadCookie(c *check.C) {
	st := state.New(nil)
	st.Lock()
	defer st.Unlock()

	id, err := hookstate.AddWorkshopCookie(st, hookstate.WorkshopCookie{
		Project:  workshop.Project{ProjectId: "p1", Path: "/p"},
		Workshop: "w1",
		User:     "alice",
	})
	c.Assert(err, check.IsNil)
	c.Assert(id, check.Not(check.HasLen), 0)

	cookies, err := hookstate.WorkshopCookies(st)
	c.Assert(err, check.IsNil)
	c.Assert(cookies, check.HasLen, 1)

	got, ok := cookies[id]
	c.Assert(ok, check.Equals, true)
	c.Check(got.Project.ProjectId, check.Equals, "p1")
	c.Check(got.Workshop, check.Equals, "w1")
	c.Check(got.User, check.Equals, "alice")
}

func (s *cookiesSuite) TestAddRotatesPriorCookieForSameWorkshop(c *check.C) {
	st := state.New(nil)
	st.Lock()
	defer st.Unlock()

	prj := workshop.Project{ProjectId: "p1", Path: "/p"}

	first, err := hookstate.AddWorkshopCookie(st, hookstate.WorkshopCookie{
		Project: prj, Workshop: "w1", User: "alice",
	})
	c.Assert(err, check.IsNil)

	second, err := hookstate.AddWorkshopCookie(st, hookstate.WorkshopCookie{
		Project: prj, Workshop: "w1", User: "alice",
	})
	c.Assert(err, check.IsNil)
	c.Check(second, check.Not(check.Equals), first)

	cookies, err := hookstate.WorkshopCookies(st)
	c.Assert(err, check.IsNil)
	c.Check(cookies, check.HasLen, 1)
	_, firstStillPresent := cookies[first]
	c.Check(firstStillPresent, check.Equals, false)
}

func (s *cookiesSuite) TestAddKeepsCookiesForOtherWorkshops(c *check.C) {
	st := state.New(nil)
	st.Lock()
	defer st.Unlock()

	prj := workshop.Project{ProjectId: "p1", Path: "/p"}

	a, err := hookstate.AddWorkshopCookie(st, hookstate.WorkshopCookie{
		Project: prj, Workshop: "w1",
	})
	c.Assert(err, check.IsNil)

	b, err := hookstate.AddWorkshopCookie(st, hookstate.WorkshopCookie{
		Project: prj, Workshop: "w2",
	})
	c.Assert(err, check.IsNil)

	cookies, err := hookstate.WorkshopCookies(st)
	c.Assert(err, check.IsNil)
	c.Check(cookies, check.HasLen, 2)
	_, hasA := cookies[a]
	_, hasB := cookies[b]
	c.Check(hasA, check.Equals, true)
	c.Check(hasB, check.Equals, true)
}

func (s *cookiesSuite) TestRemoveOnlyAffectsTargetWorkshop(c *check.C) {
	st := state.New(nil)
	st.Lock()
	defer st.Unlock()

	prj := workshop.Project{ProjectId: "p1", Path: "/p"}

	a, err := hookstate.AddWorkshopCookie(st, hookstate.WorkshopCookie{
		Project: prj, Workshop: "w1",
	})
	c.Assert(err, check.IsNil)
	b, err := hookstate.AddWorkshopCookie(st, hookstate.WorkshopCookie{
		Project: prj, Workshop: "w2",
	})
	c.Assert(err, check.IsNil)

	c.Assert(hookstate.RemoveWorkshopCookies(st, "p1", "w1"), check.IsNil)

	cookies, err := hookstate.WorkshopCookies(st)
	c.Assert(err, check.IsNil)
	c.Check(cookies, check.HasLen, 1)
	_, hasA := cookies[a]
	_, hasB := cookies[b]
	c.Check(hasA, check.Equals, false)
	c.Check(hasB, check.Equals, true)
}

func (s *cookiesSuite) TestWorkshopCookiesEmpty(c *check.C) {
	st := state.New(nil)
	st.Lock()
	defer st.Unlock()

	cookies, err := hookstate.WorkshopCookies(st)
	c.Assert(err, check.IsNil)
	c.Check(cookies, check.HasLen, 0)
}

func (s *cookiesSuite) TestContextResolvesWorkshopCookie(c *check.C) {
	st := state.New(nil)
	mgr := hookstate.NewWithoutBackend(st)

	st.Lock()
	id, err := hookstate.AddWorkshopCookie(st, hookstate.WorkshopCookie{
		Project:  workshop.Project{ProjectId: "p1", Path: "/p"},
		Workshop: "w1",
		User:     "alice",
	})
	c.Assert(err, check.IsNil)
	st.Unlock()

	ctx, err := mgr.Context(id)
	c.Assert(err, check.IsNil)
	c.Assert(ctx, check.NotNil)
	c.Check(ctx.ID(), check.Equals, id)
	c.Check(ctx.IsEphemeral(), check.Equals, true)

	cookie := ctx.Cookie()
	c.Assert(cookie, check.NotNil)
	c.Check(cookie.Workshop, check.Equals, "w1")
	c.Check(cookie.Project.ProjectId, check.Equals, "p1")
	c.Check(cookie.User, check.Equals, "alice")
}

func (s *cookiesSuite) TestContextRejectsUnknownCookie(c *check.C) {
	st := state.New(nil)
	mgr := hookstate.NewWithoutBackend(st)

	_, err := mgr.Context("not-a-real-cookie")
	c.Check(err, check.ErrorMatches, "invalid workshop cookie requested")
}
