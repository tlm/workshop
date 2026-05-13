package workshopstate

import (
	"context"
	"fmt"
	"slices"

	. "github.com/canonical/workshop/internal/overlord/handlersetup"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/workshop"
)

type WorkshopManager struct {
	backend workshop.Backend
	state   *state.State
}

func New(st *state.State, runner *state.TaskRunner) *WorkshopManager {
	manager := &WorkshopManager{
		state: st,
	}

	st.Lock()
	manager.backend = workshop.WorkshopBackend(st)
	st.Unlock()

	runner.AddHandler("download-base", OnDo(manager.doDownloadBase), nil)
	runner.AddHandler("create-workshop", OnDo(manager.doConstructWorkshop), OnUndo(manager.doRemoveWorkshop))
	runner.AddHandler("rebuild-workshop", OnDo(manager.doConstructWorkshop), OnUndo(manager.undoRebuildWorkshop))
	runner.AddHandler("start-workshop", OnDo(manager.doStart), OnUndo(manager.doStop))
	runner.AddHandler("stop-workshop", OnDo(manager.doStop), OnUndo(manager.doStart))
	runner.AddHandler("remove-workshop", OnDo(manager.doRemoveWorkshop), nil)
	runner.AddHandler("configure-timezone", OnDo(manager.doConfigureTimezone), nil)
	runner.AddHandler("mount-project", OnDo(manager.doMountProject), OnUndo(manager.undoMountProject))
	runner.AddHandler("create-workshop-storage", OnDo(manager.doCreateWorkshopStorage), OnUndo(manager.doRemoveWorkshopStorage))
	runner.AddHandler("remove-workshop-storage", OnDo(manager.doRemoveWorkshopStorage), nil)
	runner.AddHandler("remove-workshop-stash", OnDo(manager.doRemoveWorkshopStash), nil)
	runner.AddHandler("stash-workshop", OnDo(manager.doStashWorkshop), OnUndo(manager.undoStashWorkshop))
	runner.AddHandler("create-state-storage", OnDo(manager.doCreateStateStorage), OnUndo(manager.doRemoveStateStorage))
	runner.AddHandler("remove-state-storage", OnDo(manager.doRemoveStateStorage), nil)
	runner.AddHandler("install-workshop-cookie", OnDo(manager.doInstallWorkshopCookie), OnUndo(manager.undoInstallWorkshopCookie))

	return manager
}

func (m *WorkshopManager) StartUp() error {
	return nil
}

func (w *WorkshopManager) Ensure() error {
	return nil
}

// Project finds an existing project with the given ID.
func (w *WorkshopManager) Project(ctx context.Context, pId string) (workshop.Project, error) {
	user, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return workshop.Project{}, fmt.Errorf("context key %s not found", workshop.ContextUser)
	}

	projects, err := w.backend.Projects(ctx)
	if err != nil {
		return workshop.Project{}, err
	}

	idx := slices.IndexFunc(projects[user], func(p workshop.Project) bool { return p.ProjectId == pId })
	if idx == -1 {
		return workshop.Project{}, fmt.Errorf("project %q not found", pId)
	}
	return projects[user][idx], nil
}

// Loads a workshop, the state must be locked as it is used to find out the
// workshop state
func (w *WorkshopManager) Workshop(ctx context.Context, name, pId string) (*workshop.Workshop, error) {
	// project-id must be in the context for this query
	pCtx := context.WithValue(ctx, workshop.ContextProjectId, pId)

	workshop, err := w.backend.Workshop(pCtx, name)
	if err != nil {
		return nil, err
	}

	return workshop, nil
}

// Returns latest file for a workshop. The state must be locked, as listing
// projects can update project metadata.
func (w *WorkshopManager) WorkshopFile(ctx context.Context, name, pId string) (*workshop.File, error) {
	p, err := w.Project(ctx, pId)
	if err != nil {
		return nil, err
	}

	return p.Workshop(name)
}

// Returns all workshop files for a project. The state must be locked, as
// listing projects can update project metadata.
func (w *WorkshopManager) WorkshopFiles(ctx context.Context, pId string) (map[string]string, error) {
	p, err := w.Project(ctx, pId)
	if err != nil {
		return nil, err
	}

	files, err := p.ReadWorkshops()
	if err != nil {
		return files, &WorkshopFileError{err}
	}
	return files, nil
}

// WorkshopFileError wraps errors related to invalid workshop definitions or file locations.
type WorkshopFileError struct {
	err error
}

func (e *WorkshopFileError) Error() string {
	return e.err.Error()
}

func (e *WorkshopFileError) Unwrap() error {
	return e.err
}

// Returns all existing workshops for a project, the state must be
// locked as it is used to find out the workshop state.
func (w *WorkshopManager) Workshops(ctx context.Context, pId string) ([]*workshop.Workshop, error) {
	// project-id must be in the context for this query
	pCtx := context.WithValue(ctx, workshop.ContextProjectId, pId)

	workshops, err := w.backend.ProjectWorkshops(pCtx)
	if err != nil {
		return nil, err
	}

	return workshops, nil
}
