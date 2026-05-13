package workshop

import (
	"path/filepath"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/sdk"
)

func UserDataRootDir(homedir string, env map[string]string) string {
	// Use $HOME in preference of the user's home directory, if set.
	// https://specifications.freedesktop.org/basedir-spec/latest/
	if env["HOME"] != "" {
		homedir = env["HOME"]
	}

	path := filepath.Join(homedir, ".local", "share")
	dataDir := env["XDG_DATA_HOME"]
	if dataDir != "" {
		path = dataDir
	}

	return filepath.Join(path, "workshop")
}

func ProjectUserData(userDataDir, pid string) string {
	return filepath.Join(userDataDir, "id", pid)
}

func UserData(userDataDir, pid, w string) string {
	return filepath.Join(ProjectUserData(userDataDir, pid), w)
}

func SdkMountDir(userDataDir, pid, w, sdk string) string {
	return filepath.Join(UserData(userDataDir, pid, w), "mount", sdk)
}

func SdkMountHostSource(userDataDir, pid, w, sdk, plug string) string {
	return filepath.Join(SdkMountDir(userDataDir, pid, w, sdk), plug)
}

func LocalSdkDir(userDataDir, pid, w, name string) string {
	return filepath.Join(UserData(userDataDir, pid, w), "sdk", name)
}

func SketchSdkDir(userDataDir, pid, w string) string {
	return LocalSdkDir(userDataDir, pid, w, sdk.Sketch)
}

func SketchSdkCurrent(userDataDir, pid, w string) string {
	return filepath.Join(SketchSdkDir(userDataDir, pid, w), "current")
}

func SketchSdkStash(userDataDir, pid, w string) string {
	return filepath.Join(SketchSdkDir(userDataDir, pid, w), "stash")
}

func TrySdkDir(userDataDir, sdk string) string {
	return filepath.Join(userDataDir, "try", sdk)
}

func ProjectDataDir(pid string) string {
	return filepath.Join(dirs.BaseDir, "id", pid)
}

func DataDir(pid, w string) string {
	return filepath.Join(ProjectDataDir(pid), w)
}

func StateStorageDir(pid, w string) string {
	return filepath.Join(DataDir(pid, w), "state")
}

func ProjectCacheDir(pid string) string {
	return filepath.Join(dirs.CacheDir, "id", pid)
}

func CacheDir(pid, w string) string {
	return filepath.Join(ProjectCacheDir(pid), w)
}

func AptCacheDir(pid, w string) string {
	return filepath.Join(CacheDir(pid, w), "apt")
}

func SdkSourcePath(userDataDir string, project Project, w, sk string, source sdk.Source) string {
	switch source {
	case sdk.TrySource:
		return TrySdkDir(userDataDir, sk)
	case sdk.ProjectSource:
		return ProjectSdkPath(project.Path, sk)
	case sdk.SketchSource:
		return SketchSdkCurrent(userDataDir, project.ProjectId, w)
	default:
		return ""
	}
}

func SdkMount(userDataDir, pid, w string, setup sdk.Setup) Mount {
	mount := Mount{
		Name:      SdkDeviceName(setup.Name),
		Where:     sdk.SdkDir(setup.Name),
		MakeWhere: true,
		Mode:      0755,
		ReadOnly:  true,
	}

	if setup.IsVolume() {
		mount.Type = Volume
		mount.What = sdk.VolumeName(setup.Name, setup.Revision)
		return mount
	}

	mount.Type = HostWorkshop
	mount.What = filepath.Join(LocalSdkDir(userDataDir, pid, w, setup.Name), setup.Sha3_384)
	return mount
}
