package limayaml

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/lima-vm/lima/pkg/guestagent/api"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/xorcare/pointer"
	"gotest.tools/v3/assert"
)

func TestFillDefault(t *testing.T) {
	var d, y, o LimaYAML

	opts := []cmp.Option{
		// Ignore internal NetworkDeprecated.migrated field
		cmpopts.IgnoreUnexported(NetworkDeprecated{}),
		// Consider nil slices and empty slices to be identical
		cmpopts.EquateEmpty(),
	}

	arch := AARCH64
	if runtime.GOARCH == "amd64" {
		arch = X8664
	}

	hostHome, err := os.UserHomeDir()
	assert.NilError(t, err)
	limaHome, err := dirnames.LimaDir()
	assert.NilError(t, err)
	user, err := osutil.LimaUser(false)
	assert.NilError(t, err)

	guestHome := fmt.Sprintf("/home/%s.linux", user.Username)
	instName := "instance"
	instDir := filepath.Join(limaHome, instName)
	filePath := filepath.Join(instDir, filenames.LimaYAML)

	// Builtin default values
	builtin := LimaYAML{
		Arch:   pointer.String(arch),
		CPUs:   pointer.Int(4),
		Memory: pointer.String("4GiB"),
		Disk:   pointer.String("100GiB"),
		Containerd: Containerd{
			System:   pointer.Bool(false),
			User:     pointer.Bool(true),
			Archives: defaultContainerdArchives(),
		},
		SSH: SSH{
			LocalPort:         pointer.Int(0),
			LoadDotSSHPubKeys: pointer.Bool(true),
			ForwardAgent:      pointer.Bool(false),
		},
		Firmware: Firmware{
			LegacyBIOS: pointer.Bool(false),
		},
		Video: Video{
			Display: pointer.String("none"),
		},
		UseHostResolver:   pointer.Bool(true),
		PropagateProxyEnv: pointer.Bool(true),
	}

	defaultPortForward := PortForward{
		GuestIP:        api.IPv4loopback1,
		GuestPortRange: [2]int{1, 65535},
		HostIP:         api.IPv4loopback1,
		HostPortRange:  [2]int{1, 65535},
		Proto:          TCP,
	}

	// ------------------------------------------------------------------------------------
	// Builtin defaults are set when y is (mostly) empty

	// All these slices and maps are empty in "builtin". Add minimal entries here to see that
	// their values are retained and defaults for their fields are applied correctly.
	y = LimaYAML{
		Mounts: []Mount{
			{Location: "/tmp"},
		},
		Provision: []Provision{
			{Script: "#!/bin/true"},
		},
		Probes: []Probe{
			{Script: "#!/bin/false"},
		},
		Networks: []Network{
			{Lima: "shared"},
		},
		DNS: []net.IP{
			net.ParseIP("1.0.1.0"),
		},
		PortForwards: []PortForward{
			{},
			{GuestPort: 80},
			{GuestPort: 8080, HostPort: 8888},
			{
				GuestSocket: "{{.Home}} | {{.UID}} | {{.User}}",
				HostSocket:  "{{.Home}} | {{.Dir}} | {{.Name}} | {{.UID}} | {{.User}}",
			},
		},
		Env: map[string]string{
			"ONE": "Eins",
		},
	}

	expect := builtin
	expect.Mounts = y.Mounts
	expect.Mounts[0].Writable = pointer.Bool(false)
	expect.Mounts[0].SSHFS.FollowSymlinks = pointer.Bool(false)
	// Only missing Mounts field is Writable, and the default value is also the null value: false

	expect.Provision = y.Provision
	expect.Provision[0].Mode = ProvisionModeSystem

	expect.Probes = y.Probes
	expect.Probes[0].Mode = ProbeModeReadiness
	expect.Probes[0].Description = "user probe 1/1"

	expect.Networks = y.Networks
	expect.Networks[0].MACAddress = MACAddress(fmt.Sprintf("%s#%d", filePath, 0))
	expect.Networks[0].Interface = "lima0"

	expect.DNS = y.DNS
	expect.PortForwards = []PortForward{
		defaultPortForward,
		defaultPortForward,
		defaultPortForward,
		defaultPortForward,
	}
	// Setting GuestPort and HostPort for DeepEqual(), but they are not supposed to be used
	// after FillDefault() has been called and the ...PortRange fields have been set.
	expect.PortForwards[1].GuestPort = 80
	expect.PortForwards[1].GuestPortRange = [2]int{80, 80}
	expect.PortForwards[1].HostPortRange = expect.PortForwards[1].GuestPortRange

	expect.PortForwards[2].GuestPort = 8080
	expect.PortForwards[2].GuestPortRange = [2]int{8080, 8080}
	expect.PortForwards[2].HostPort = 8888
	expect.PortForwards[2].HostPortRange = [2]int{8888, 8888}

	expect.PortForwards[3].GuestSocket = fmt.Sprintf("%s | %s | %s", guestHome, user.Uid, user.Username)
	expect.PortForwards[3].HostSocket = fmt.Sprintf("%s | %s | %s | %s | %s", hostHome, instDir, instName, user.Uid, user.Username)

	expect.Env = y.Env

	FillDefault(&y, &LimaYAML{}, &LimaYAML{}, filePath)
	assert.DeepEqual(t, &y, &expect, opts...)

	filledDefaults := y

	// ------------------------------------------------------------------------------------
	// User-provided defaults should override any builtin defaults

	// Choose values that are different from the "builtin" defaults
	d = LimaYAML{
		Arch:   pointer.String("unknown"),
		CPUs:   pointer.Int(7),
		Memory: pointer.String("5GiB"),
		Disk:   pointer.String("105GiB"),
		Containerd: Containerd{
			System: pointer.Bool(true),
			User:   pointer.Bool(false),
			Archives: []File{
				{Location: "/tmp/nerdctl.tgz"},
			},
		},
		SSH: SSH{
			LocalPort:         pointer.Int(888),
			LoadDotSSHPubKeys: pointer.Bool(false),
			ForwardAgent:      pointer.Bool(true),
		},
		Firmware: Firmware{
			LegacyBIOS: pointer.Bool(true),
		},
		Video: Video{
			Display: pointer.String("cocoa"),
		},
		UseHostResolver:   pointer.Bool(false),
		PropagateProxyEnv: pointer.Bool(false),

		Mounts: []Mount{
			{
				Location: "/var/log",
				Writable: pointer.Bool(false),
			},
		},
		Provision: []Provision{
			{
				Script: "#!/bin/true",
				Mode:   ProvisionModeUser,
			},
		},
		Probes: []Probe{
			{
				Script:      "#!/bin/false",
				Mode:        ProbeModeReadiness,
				Description: "User Probe",
			},
		},
		Networks: []Network{
			{
				VNL:        "/tmp/vde.ctl",
				SwitchPort: 65535,
				MACAddress: "11:22:33:44:55:66",
				Interface:  "def0",
			},
		},
		DNS: []net.IP{
			net.ParseIP("1.1.1.1"),
		},
		PortForwards: []PortForward{{
			GuestIP:        api.IPv4loopback1,
			GuestPort:      80,
			GuestPortRange: [2]int{80, 80},
			HostIP:         api.IPv4loopback1,
			HostPort:       80,
			HostPortRange:  [2]int{80, 80},
			Proto:          TCP,
		}},
		Env: map[string]string{
			"ONE": "one",
			"TWO": "two",
		},
	}

	expect = d
	// Also verify that archive arch is filled in
	expect.Containerd.Archives[0].Arch = *d.Arch
	expect.Mounts[0].SSHFS.FollowSymlinks = pointer.Bool(false)

	y = LimaYAML{}
	FillDefault(&y, &d, &LimaYAML{}, filePath)
	assert.DeepEqual(t, &y, &expect, opts...)

	// ------------------------------------------------------------------------------------
	// User-provided defaults should not override user-provided config values

	y = filledDefaults
	y.DNS = []net.IP{net.ParseIP("8.8.8.8")}

	expect = y

	expect.Provision = append(y.Provision, d.Provision...)
	expect.Probes = append(y.Probes, d.Probes...)
	expect.PortForwards = append(y.PortForwards, d.PortForwards...)
	expect.Containerd.Archives = append(y.Containerd.Archives, d.Containerd.Archives...)

	// Mounts and Networks start with lowest priority first, so higher priority entries can overwrite
	expect.Mounts = append(d.Mounts, y.Mounts...)
	expect.Networks = append(d.Networks, y.Networks...)

	// d.DNS will be ignored, and not appended to y.DNS

	// "TWO" does not exist in filledDefaults.Env, so is set from d.Env
	expect.Env["TWO"] = d.Env["TWO"]

	FillDefault(&y, &d, &LimaYAML{}, filePath)
	assert.DeepEqual(t, &y, &expect, opts...)

	// ------------------------------------------------------------------------------------
	// User-provided overrides should override user-provided config settings

	o = LimaYAML{
		Arch:   pointer.String(arch),
		CPUs:   pointer.Int(12),
		Memory: pointer.String("7GiB"),
		Disk:   pointer.String("117GiB"),
		Containerd: Containerd{
			System: pointer.Bool(true),
			User:   pointer.Bool(false),
			Archives: []File{
				{
					Arch:     arch,
					Location: "/tmp/nerdctl.tgz",
					Digest:   "$DIGEST",
				},
			},
		},
		SSH: SSH{
			LocalPort:         pointer.Int(4433),
			LoadDotSSHPubKeys: pointer.Bool(true),
			ForwardAgent:      pointer.Bool(true),
		},
		Firmware: Firmware{
			LegacyBIOS: pointer.Bool(true),
		},
		Video: Video{
			Display: pointer.String("cocoa"),
		},
		UseHostResolver:   pointer.Bool(false),
		PropagateProxyEnv: pointer.Bool(false),

		Mounts: []Mount{
			{
				Location: "/var/log",
				Writable: pointer.Bool(true),
				SSHFS:    SSHFS{FollowSymlinks: pointer.Bool(true)},
			},
		},
		Provision: []Provision{
			{
				Script: "#!/bin/true",
				Mode:   ProvisionModeSystem,
			},
		},
		Probes: []Probe{
			{
				Script:      "#!/bin/false",
				Mode:        ProbeModeReadiness,
				Description: "Another Probe",
			},
		},
		Networks: []Network{
			{
				Lima:       "shared",
				MACAddress: "10:20:30:40:50:60",
				Interface:  "def1",
			},
			{
				Lima:      "bridged",
				Interface: "def0",
			},
		},
		DNS: []net.IP{
			net.ParseIP("2.2.2.2"),
		},
		PortForwards: []PortForward{{
			GuestIP:        api.IPv4loopback1,
			GuestPort:      88,
			GuestPortRange: [2]int{88, 88},
			HostIP:         api.IPv4loopback1,
			HostPort:       8080,
			HostPortRange:  [2]int{8080, 8080},
			Proto:          TCP,
		}},
		Env: map[string]string{
			"TWO":   "deux",
			"THREE": "trois",
		},
	}

	y = filledDefaults

	expect = o

	expect.Provision = append(append(o.Provision, y.Provision...), d.Provision...)
	expect.Probes = append(append(o.Probes, y.Probes...), d.Probes...)
	expect.PortForwards = append(append(o.PortForwards, y.PortForwards...), d.PortForwards...)
	expect.Containerd.Archives = append(append(o.Containerd.Archives, y.Containerd.Archives...), d.Containerd.Archives...)

	// o.Mounts just makes d.Mounts[0] writable because the Location matches
	expect.Mounts = append(d.Mounts, y.Mounts...)
	expect.Mounts[0].Writable = pointer.Bool(true)
	expect.Mounts[0].SSHFS.FollowSymlinks = pointer.Bool(true)

	// o.Networks[1] is overriding the d.Networks[0].Lima entry for the "def0" interface
	expect.Networks = append(append(d.Networks, y.Networks...), o.Networks[0])
	expect.Networks[0].Lima = o.Networks[1].Lima
	expect.Networks[0].VNL = ""
	expect.Networks[0].SwitchPort = 0

	// Only highest prio DNS are retained
	expect.DNS = o.DNS

	// ONE remains from filledDefaults.Env; the rest are set from o
	expect.Env["ONE"] = y.Env["ONE"]

	FillDefault(&y, &d, &o, filePath)
	assert.DeepEqual(t, &y, &expect, opts...)
}
