package workshop

import (
	"os"
	"path/filepath"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/osutil/sys"
)

var DefaultDevices = defaultDevices

type MountType int

const (
	HostWorkshop MountType = iota
	WorkshopWorkshop
	Volume
)

type ProxyTarget struct {
	Address  string
	Protocol string
}

type ProxyDirection int

const (
	HostToWorkshop ProxyDirection = iota
	WorkshopToHost
)

type ProxyEntry struct {
	Name      string
	Connect   ProxyTarget
	Listen    ProxyTarget
	Direction ProxyDirection
}

func (p *ProxyEntry) Equal(other *ProxyEntry) bool {
	if p == nil || other == nil {
		return p == other
	}

	return *p == *other
}

type Camera struct {
	Name string `json:"name"`
}

type Mount struct {
	Name      string      `json:"name"`
	Type      MountType   `json:"type"`
	What      string      `json:"what"`
	MakeWhat  bool        `json:"make-what,omitempty"`
	Where     string      `json:"where"`
	MakeWhere bool        `json:"make-where,omitempty"`
	Mode      os.FileMode `json:"mode,omitempty"`
	Owner     sys.UserID  `json:"owner,omitempty"`
	Group     sys.GroupID `json:"group,omitempty"`
	ReadOnly  bool        `json:"readonly"`
}

type Tunnel struct {
	ProxyEntry
}

type SshAgent struct {
	ProxyEntry
}

func (s *SshAgent) Equal(other *SshAgent) bool {
	if s == nil || other == nil {
		return s == other
	}

	return *s == *other
}

type Desktop struct {
	Wayland *ProxyEntry
	X11     *ProxyEntry
}

func (d *Desktop) Equal(other *Desktop) bool {
	if d == nil || other == nil {
		return d == other
	}

	return d.Wayland.Equal(other.Wayland) && d.X11.Equal(other.X11)
}

type Gpu struct {
	Name string
}

type SdkProfile struct {
	Sdk string

	Camera  *Camera
	Mounts  map[string]Mount
	Tunnels []Tunnel
	Agent   *SshAgent
	Gpu     *Gpu
	Desktop *Desktop
}

func NewSdkProfile(sdkName string) SdkProfile {
	return SdkProfile{
		Sdk:    sdkName,
		Mounts: make(map[string]Mount),
	}
}

func defaultDevices(pid, w string) ([]Mount, []ProxyEntry) {
	mounts := []Mount{{
		Name:     "workshop.workshopctl",
		Type:     HostWorkshop,
		What:     filepath.Join(dirs.ExecDir, "workshopctl"),
		Where:    "/usr/bin/workshopctl",
		ReadOnly: true,
	}, {
		Name:  "cache.apt",
		Type:  HostWorkshop,
		What:  AptCacheDir(pid, w),
		Where: dirs.AptCacheDir,
	}}

	socketHost := dirs.SocketPath + ".untrusted"
	socketWorkshop := filepath.Join(dirs.WorkshopRunDir, filepath.Base(socketHost))
	proxies := []ProxyEntry{{
		Name:      "workshop.socket",
		Connect:   ProxyTarget{Address: socketHost, Protocol: "unix"},
		Listen:    ProxyTarget{Address: socketWorkshop, Protocol: "unix"},
		Direction: WorkshopToHost,
	}}

	return mounts, proxies
}

func FakeDefaultDevices(f func(pid, w string) ([]Mount, []ProxyEntry)) func() {
	oldDefault := DefaultDevices
	DefaultDevices = f
	return func() { DefaultDevices = oldDefault }
}
