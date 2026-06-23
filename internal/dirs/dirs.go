// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package dirs

import (
	"fmt"
	"os"
	"path/filepath"
)

// defaultBaseDir is the Workshop directory used if $WORKSHOP is not set. It is
// created by the daemon ("workshopd run") if it doesn't exist, and also used by
// the workshop client.
const defaultBaseDir = "/var/lib/workshop"

// defaultCacheDir is the Workshop directory used if $WORKSHOP_CACHE is not set. It is
// created by the daemon ("workshopd run") if it doesn't exist.
const defaultCacheDir = "/var/cache/workshop"

// Variables for paths inside a workshop
var (
	// base directory inside a workshop
	WorkshopBaseDir = defaultBaseDir

	// SDKs directory to install an SDK in a workshop
	WorkshopSdksDir = filepath.Join(WorkshopBaseDir, "sdk")

	// Base directory for the state storage
	WorkshopStateDir = filepath.Join(WorkshopBaseDir, "state")

	// Base directory for the SDK state storage
	WorkshopSdkStateDir = filepath.Join(WorkshopStateDir, "sdk")

	// Run directory inside workshop
	WorkshopRunDir = filepath.Join(WorkshopBaseDir, "run")

	// Directory of workshopctl shims that arm SDK binaries with secrets. It is
	// prepended to PATH so a shim shadows the real binary it wraps.
	WorkshopBinDir = filepath.Join(WorkshopRunDir, "bin")

	// Directory of secret key registry entries. Each file is named by a key and
	// records the context (sdk, binary, plug, env-mapping) that key resolves to.
	WorkshopSecretsDir = filepath.Join(WorkshopRunDir, "secrets")

	// Directory for actions inside workshop
	WorkshopActionsDir = filepath.Join(WorkshopRunDir, "actions")

	// Cache directory for deb packages
	AptCacheDir = "/var/cache/apt/archives"
)

// Variables for workshopd (host paths)
var (
	// Base directory for workshopd
	BaseDir string
	// Cache directory for workshopd
	CacheDir string
	// Work directory
	ExecDir string
	// The directory to store downloaded base images and associated metadata
	BaseDownloads string
	// The directory to store downloaded SDKs
	SdkDownloads string
	// Path to the daemon's unix socket
	SocketPath string
	// State lock file
	WorkshopStateLockFile string
	// Base for the XDG runtime directory of a host user
	XdgRuntimeDirBase string
	// Run directory
	WorkshopdRunDir string
	// Locks directory
	WorkshopdLocksDir string
	// Certificates
	WorkshopTlsDir string
)

func getEnvPaths() (workshopdDir, cacheDir, socketPath string) {
	workshopdDir = os.Getenv("WORKSHOP")
	if workshopdDir == "" {
		workshopdDir = defaultBaseDir
	}
	cacheDir = os.Getenv("WORKSHOP_CACHE")
	if cacheDir == "" {
		cacheDir = defaultCacheDir
	}
	socketPath = os.Getenv("WORKSHOP_SOCKET")
	if socketPath == "" {
		socketPath = filepath.Join(workshopdDir, "workshop.socket")
	}
	return workshopdDir, cacheDir, socketPath
}

func init() {
	var err error
	var execPath string
	execPath, err = os.Executable()
	if err != nil {
		panic("cannot get working directory")
	}

	ExecDir = filepath.Dir(execPath)
	XdgRuntimeDirBase = "/run/user"
	BaseDir, CacheDir, SocketPath = getEnvPaths()
	SetRootDir(BaseDir)
	SetCacheDir(CacheDir)
}

func SetRootDir(rootdir string) {
	if !filepath.IsAbs(rootdir) {
		panic(fmt.Sprintf("cannot set root dir: path %q is not absolute", rootdir))
	}
	BaseDir = rootdir

	WorkshopStateLockFile = filepath.Join(BaseDir, "state.lock")
	WorkshopTlsDir = filepath.Join(BaseDir, "tls")
	WorkshopdRunDir = filepath.Join(BaseDir, "/run/workshopd")
	WorkshopdLocksDir = filepath.Join(WorkshopdRunDir, "locks")
}

func SetCacheDir(cachedir string) {
	if !filepath.IsAbs(cachedir) {
		panic(fmt.Sprintf("cannot set cache dir: path %q is not absolute", cachedir))
	}
	CacheDir = cachedir

	BaseDownloads = filepath.Join(CacheDir, "base")
	SdkDownloads = filepath.Join(CacheDir, "sdk")
}

func CreateDirs() error {
	if err := os.MkdirAll(BaseDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(CacheDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(BaseDownloads, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(SdkDownloads, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(WorkshopdRunDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(WorkshopdLocksDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(WorkshopTlsDir, 0755); err != nil {
		return err
	}
	return nil
}
