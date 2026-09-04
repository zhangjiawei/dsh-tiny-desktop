package core

import (
	"net"
	"path/filepath"
	"testing"
)

func TestVisibleDefaultsAndLegacyLaunch(t *testing.T) {
	s := Defaults()
	if s.Registry != "https://registry.npmmirror.com" || s.Command != DefaultCommand {
		t.Fatal("missing visible defaults")
	}
	if _, err := ParseCommand(s.Command); err != nil {
		t.Fatal(err)
	}
	p, _ := NewPaths(t.TempDir())
	if err := AtomicWrite(filepath.Join(p.Root, "settings.json"), []byte(`{"command":"","registry":"https://registry.npmjs.org"}`), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := p.LoadSettings()
	if err != nil || got.Command != ManagedCommand || got.Registry != "https://registry.npmjs.org" {
		t.Fatal("legacy launch/preferences changed", got, err)
	}
	r := Runtime{Node: "private-node", CLI: "private-dsh"}
	installer := Installer{Paths: p, Settings: got}
	bin, args, err := installer.launchCommand(r)
	if err != nil || bin != r.Node || len(args) != 2 || args[0] != r.CLI || args[1] != "web" {
		t.Fatal("managed command did not use private runtime", bin, args, err)
	}
}

func TestOccupiedPortSnapshotTracksActiveLaunch(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	preferred := listener.Addr().(*net.TCPAddr).Port
	actual, err := CandidatePort(preferred)
	if err != nil || actual == preferred {
		t.Fatal("occupied port was not replaced", err)
	}
	p, _ := NewPaths(t.TempDir())
	s := Defaults()
	s.Port = preferred
	m := NewManager(p, s)
	m.phase = "running"
	m.port = actual
	m.cancel = func() {}
	snap := m.Snapshot()
	if !snap.PortChanged || snap.Port != actual || snap.PreferredPort != preferred {
		t.Fatal("conflict banner lost actual port", snap)
	}
	s.Port = actual
	if err = m.Configure(s); err != nil {
		t.Fatal(err)
	}
	if !m.Snapshot().PortChanged {
		t.Fatal("pending setting changed active conflict result")
	}
	m.phase = "stopped"
	if m.Snapshot().PortChanged {
		t.Fatal("stopped service claims a live random port")
	}
}
