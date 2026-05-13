package workshopstate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"gopkg.in/tomb.v2"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/osutil/sys"
	. "github.com/canonical/workshop/internal/overlord/handlersetup"
	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/progress"
	"github.com/canonical/workshop/internal/revert"
	"github.com/canonical/workshop/internal/workshop"
)

// WorkshopCookieScript is the path inside the workshop where the long-lived
// WORKSHOP_COOKIE env var is exported for interactive shells.
const WorkshopCookieScript = "/etc/profile.d/workshop-cookie.sh"

var StopLogInterval = 30 * time.Second

var StopWorkshop = (workshop.Backend).StopWorkshop

func (m *WorkshopManager) doDownloadBase(task *state.Task, tomb *tomb.Tomb) error {
	user, project, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	image, err := WorkshopBase(task.Change(), w)
	st.Unlock()
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	reporter := &progress.Reporter{
		Name: task.ID(),
		Report: func(label string, done, total int64) {
			st.Lock()
			task.SetProgress(label, done, total)
			st.Unlock()
		},
	}

	return m.backend.DownloadBase(ctx, image, reporter)
}

func (m *WorkshopManager) doConstructWorkshop(task *state.Task, tomb *tomb.Tomb) error {
	user, project, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	st := task.State()

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	st.Lock()
	wf, err := WorkshopFile(task, w)
	st.Unlock()
	if err != nil {
		return err
	}

	st.Lock()
	lastIntact, err := MaybeLastIntactSdk(task)
	st.Unlock()
	if err != nil {
		return err
	}

	st.Lock()
	snapshot, err := WorkshopSnapshot(task.Change(), w, lastIntact)
	st.Unlock()
	if err != nil {
		return err
	}

	rev := revert.New()
	defer rev.Fail()

	if task.Kind() == "create-workshop" {
		cleanupCtx := context.WithoutCancel(ctx)
		rev.Add(func() {
			cleanupCtx, cancel := context.WithTimeout(cleanupCtx, 30*time.Second)
			defer cancel()

			// This may fail if the first workshop launch has failed for some
			// reason; it is safe to ignore the error in that case.
			if reverr := m.backend.RemoveWorkshop(cleanupCtx, w); reverr != nil {
				logger.Noticef("On doConstructWorkshop: cannot remove %q workshop on cleanup: %v", w, reverr)
			}
		})
	}

	if err := m.backend.LaunchOrRebuildWorkshop(ctx, wf, *snapshot); err != nil {
		return err
	}

	if snapshot.IsBase() {
		// Create workshop base and run directories
		fs, err := m.backend.WorkshopFs(ctx, wf.Name)
		if err != nil {
			return err
		}
		defer fs.Close()
		if err = fs.MkdirAll(dirs.WorkshopRunDir, 0755); err != nil {
			return err
		}
	}

	rev.Success()
	return nil
}

func (m *WorkshopManager) undoRebuildWorkshop(task *state.Task, tomb *tomb.Tomb) error {
	// The undo handler for stash-workshop rolls back both stash-workshop
	// and rebuild-workshop.
	return nil
}

func (m *WorkshopManager) doCreateWorkshopStorage(task *state.Task, tomb *tomb.Tomb) error {
	username, prj, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	// The workshop's root user needs write access to the apt cache.
	// We use the client user (mapped to the workshop user) for simplicity.
	// Another approach would be to add shift=true to the mount device,
	// but it's risky in terms of security (e.g. setuid binaries).
	user, err := osutil.UserLookup(username)
	if err != nil {
		return err
	}
	uid, gid, err := osutil.UidGid(user)
	if err != nil {
		return err
	}

	aptCache := workshop.AptCacheDir(prj.ProjectId, w)
	if err := os.MkdirAll(aptCache, 0755); err != nil {
		return err
	}
	if err = sys.ChownPath(aptCache, uid, gid); err != nil {
		return &os.PathError{Op: "chown", Path: aptCache, Err: err}
	}

	return nil
}

func (m *WorkshopManager) doRemoveWorkshopStorage(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	if err := m.cleanupWorkshopUserData(user, prj.ProjectId, w); err != nil {
		task.State().Lock()
		task.Logf("cannot remove %q workshop user data: %v", w, err)
		task.State().Unlock()
	}

	if err := m.cleanupWorkshopCache(prj.ProjectId, w); err != nil {
		task.State().Lock()
		task.Logf("cannot remove %q workshop cache: %v", w, err)
		task.State().Unlock()
	}

	if err := m.cleanupWorkshopData(prj.ProjectId, w); err != nil {
		task.State().Lock()
		task.Logf("cannot remove %q workshop data: %v", w, err)
		task.State().Unlock()
	}

	return nil
}

func (m *WorkshopManager) cleanupWorkshopUserData(user, projectId, w string) error {
	usr, env, err := osutil.UserAndEnv(user)
	if err != nil {
		return err
	}

	userDataDir := workshop.UserDataRootDir(usr.HomeDir, env)
	workshopUserData := workshop.UserData(userDataDir, projectId, w)
	if err := os.RemoveAll(workshopUserData); err != nil {
		return err
	}

	return removeIfEmpty(workshop.ProjectUserData(userDataDir, projectId))
}

func (m *WorkshopManager) cleanupWorkshopCache(projectId, w string) error {
	cache := workshop.CacheDir(projectId, w)
	if err := os.RemoveAll(cache); err != nil {
		return err
	}

	return removeIfEmpty(workshop.ProjectCacheDir(projectId))
}

func (m *WorkshopManager) cleanupWorkshopData(projectId, w string) error {
	workshopData := workshop.DataDir(projectId, w)
	if err := os.RemoveAll(workshopData); err != nil {
		return err
	}

	return removeIfEmpty(workshop.ProjectDataDir(projectId))
}

func removeIfEmpty(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTEMPTY) {
		return nil
	}
	return err
}

func (m *WorkshopManager) doConfigureTimezone(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	timezone, err := osutil.Timezone()
	if err != nil {
		st := task.State()
		st.Lock()
		defer st.Unlock()
		st.Warnf("cannot determine system time zone: %v", err)
		return nil
	}

	// Set /etc/localtime in workshop. Ubuntu 22.04 and below use a patched
	// systemd that will also write to /etc/timezone.
	args := workshop.Execution{
		ExecArgs: workshop.ExecArgs{
			Command: []string{
				"timedatectl",
				"set-timezone",
				timezone,
			},
			WorkDir: "/",
			Timeout: time.Minute,
		},
	}
	exectx, err := m.backend.Exec(ctx, w, &args)
	if err != nil {
		return err
	}
	if err := exectx.WaitExecution(ctx); err != nil {
		return err
	}

	// Ensure /etc/timezone is updated (for Ubuntu 24.04 and above). It's
	// looking like Ubuntu 26.04 will remove /etc/timezone, so this command
	// only affects Ubuntu 24.04. It can be removed when we drop support.
	args = workshop.Execution{
		ExecArgs: workshop.ExecArgs{
			Command: []string{
				"dpkg-reconfigure",
				"--frontend=noninteractive",
				"tzdata",
			},
			WorkDir: "/",
			Timeout: time.Minute,
		},
	}
	exectx, err = m.backend.Exec(ctx, w, &args)
	if err != nil {
		return err
	}
	err = exectx.WaitExecution(ctx)
	var errExec *workshop.ErrExec
	if errors.As(err, &errExec) && errExec.Status == osutil.CommandNotFound {
		// If dpkg-reconfigure doesn't exist, /etc/timezone probably
		// doesn't either.
		err = nil
	}
	return err
}

func (m *WorkshopManager) doMountProject(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	// Configure workshop core properties: project directory
	var prjMount = workshop.Mount{
		Name:  workshop.ConfigProjectPathDevice,
		Type:  workshop.HostWorkshop,
		What:  prj.Path,
		Where: workshop.WorkshopProjectPath,
	}
	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	return m.backend.AddWorkshopMount(ctx, w, prjMount)
}

func (m *WorkshopManager) undoMountProject(task *state.Task, tomb *tomb.Tomb) error {
	// No need to undo because the mount will be removed with the workshop
	return nil
}

func (m *WorkshopManager) doStart(task *state.Task, tomb *tomb.Tomb) error {
	user, project, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	st := task.State()
	st.Lock()
	task.Set("force", true)
	st.Unlock()

	return m.backend.StartWorkshop(ctx, w)
}

func (m *WorkshopManager) doStop(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	var force bool
	st := task.State()
	st.Lock()
	// false is by default
	_ = task.Get("force", &force)
	st.Unlock()

	var stopped = make(chan error)
	stopctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		// LXD has an internal timeout (30 seconds) for the operation,
		// if exceeded, the dealine error will be returned
		stopped <- StopWorkshop(m.backend, stopctx, w, force)
	}()

	for {
		select {
		case err = <-stopped:
			return err
		case <-time.After(StopLogInterval):
			st.Lock()
			task.Logf("Still waiting for %q to stop; no change in the last 30 seconds...", w)
			st.Unlock()
		}
	}
}

func (m *WorkshopManager) doRemoveWorkshop(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	if err := m.backend.RemoveWorkshop(ctx, w); err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()
	if err := hookstate.RemoveWorkshopCookies(st, prj.ProjectId, w); err != nil {
		task.Logf("cannot remove %q workshop cookies: %v", w, err)
	}
	return nil
}

// doInstallWorkshopCookie registers a long-lived workshop cookie in state and
// writes it into the workshop's /etc/profile.d so interactive shells inside
// the workshop can authenticate to workshopd via workshopctl.
func (m *WorkshopManager) doInstallWorkshopCookie(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	st := task.State()

	st.Lock()
	cookieID, err := hookstate.AddWorkshopCookie(st, hookstate.WorkshopCookie{
		Project:  *prj,
		Workshop: w,
		User:     user,
	})
	st.Unlock()
	if err != nil {
		return fmt.Errorf("cannot register workshop cookie: %w", err)
	}

	rev := revert.New()
	defer rev.Fail()
	rev.Add(func() {
		st.Lock()
		defer st.Unlock()
		if cleanupErr := hookstate.RemoveWorkshopCookies(st, prj.ProjectId, w); cleanupErr != nil {
			task.Logf("cannot remove %q workshop cookie on cleanup: %v", w, cleanupErr)
		}
	})

	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	if err := writeWorkshopCookieScript(ctx, m.backend, w, cookieID); err != nil {
		return fmt.Errorf("cannot install workshop cookie: %w", err)
	}

	rev.Success()
	return nil
}

// undoInstallWorkshopCookie removes the cookie from state. The workshop
// filesystem is typically already gone by the time undo runs (because
// remove-workshop has been undone), so a best-effort filesystem cleanup is
// good enough.
func (m *WorkshopManager) undoInstallWorkshopCookie(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	cleanupErr := hookstate.RemoveWorkshopCookies(st, prj.ProjectId, w)
	st.Unlock()
	if cleanupErr != nil {
		task.State().Lock()
		task.Logf("cannot remove %q workshop cookie on undo: %v", w, cleanupErr)
		task.State().Unlock()
	}

	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()
	_ = removeWorkshopCookieScript(ctx, m.backend, w)
	return nil
}

func writeWorkshopCookieScript(ctx context.Context, backend workshop.Backend, w, cookieID string) error {
	fs, err := backend.WorkshopFs(ctx, w)
	if err != nil {
		return err
	}
	defer fs.Close()

	if err := fs.MkdirAll("/etc/profile.d", 0755); err != nil {
		return err
	}
	content := ": \"${WORKSHOP_COOKIE:=" + shellQuote(cookieID) + "}\"\nexport WORKSHOP_COOKIE\n"
	return fs.AtomicWriteTo(strings.NewReader(content), WorkshopCookieScript, 0644)
}

func removeWorkshopCookieScript(ctx context.Context, backend workshop.Backend, w string) error {
	fs, err := backend.WorkshopFs(ctx, w)
	if err != nil {
		return err
	}
	defer fs.Close()
	return fs.RemoveIfExists(WorkshopCookieScript)
}

// shellQuote single-quotes a string for use in a POSIX shell `export` line.
// The cookie is a hex token so this is belt-and-braces.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (m *WorkshopManager) doRemoveWorkshopStash(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	if err := m.backend.RemoveWorkshopStash(ctx, w); err != nil {
		task.State().Lock()
		task.Logf("cannot remove %q workshop stash: %v", w, err)
		task.State().Unlock()
	}

	return nil
}

func (m *WorkshopManager) doStashWorkshop(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	if err = m.backend.StashWorkshop(ctx, w); err != nil {
		return err
	}
	return nil
}

func (m *WorkshopManager) undoStashWorkshop(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	if err = m.backend.UnstashWorkshop(ctx, w); err != nil {
		return err
	}

	return m.backend.RemoveWorkshopStash(ctx, w)
}

func (m *WorkshopManager) doCreateStateStorage(task *state.Task, tomb *tomb.Tomb) error {
	username, prj, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	// The workshop's root user needs write access to the state storage.
	// We use the client user (mapped to the workshop user) for simplicity.
	// Another approach would be to add shift=true to the mount device,
	// but it's risky in terms of security (e.g. setuid binaries).
	user, err := osutil.UserLookup(username)
	if err != nil {
		return err
	}
	uid, gid, err := osutil.UidGid(user)
	if err != nil {
		return err
	}

	storage := workshop.StateStorageDir(prj.ProjectId, w)
	if err := os.MkdirAll(storage, 0755); err != nil {
		return err
	}
	if err = sys.ChownPath(storage, uid, gid); err != nil {
		return &os.PathError{Op: "chown", Path: storage, Err: err}
	}

	return nil
}

func (m *WorkshopManager) doRemoveStateStorage(task *state.Task, tomb *tomb.Tomb) error {
	_, prj, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	storage := workshop.StateStorageDir(prj.ProjectId, w)
	if err := os.RemoveAll(storage); err != nil {
		task.State().Lock()
		task.Logf("cannot remove %q workshop state storage: %v", w, err)
		task.State().Unlock()
	}

	return nil
}
