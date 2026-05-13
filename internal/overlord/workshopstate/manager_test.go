package workshopstate_test

import (
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/overlord/workshopstate"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
	"github.com/canonical/workshop/internal/workshop/fakebackend"
)

type managerSuite struct {
	state   *state.State
	backend workshop.Backend
	runner  *state.TaskRunner
	manager *workshopstate.WorkshopManager
}

var _ = check.Suite(&managerSuite{})

func (s *managerSuite) SetUpTest(c *check.C) {
	var err error
	s.state = state.New(nil)
	s.backend, err = fakebackend.New(c.MkDir())
	c.Assert(err, check.IsNil)
	workshop.ReplaceBackend(s.state, s.backend)
	s.runner = state.NewTaskRunner(s.state)
	s.manager = workshopstate.New(s.state, s.runner)
}

func (s *managerSuite) TestAddHandlers(c *check.C) {
	workshopstate.New(s.state, s.runner)

	c.Assert(s.runner.KnownTaskKinds(), testutil.DeepUnsortedMatches, []string{
		"download-base",
		"create-workshop",
		"rebuild-workshop",
		"start-workshop",
		"stop-workshop",
		"remove-workshop",
		"configure-timezone",
		"mount-project",
		"create-workshop-storage",
		"remove-workshop-storage",
		"remove-workshop-stash",
		"stash-workshop",
		"create-state-storage",
		"remove-state-storage",
		"install-workshop-cookie",
	})
}
