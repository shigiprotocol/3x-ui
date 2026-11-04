package aether

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// readyTimeout bounds how long the readiness probe waits for a core's dial port
// to accept a connection; the process keeps running regardless, because Aether
// may still be scanning for an endpoint.
const readyTimeout = 20 * time.Second

// ProfileStatus is a snapshot of one running core the panel renders.
type ProfileStatus struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Running  bool     `json:"running"`
	Ready    bool     `json:"ready"`
	Port     int      `json:"port"`
	DialPort int      `json:"dialPort"`
	Pid      int      `json:"pid"`
	LastErr  string   `json:"lastErr"`
	Settings Settings `json:"settings"`
}

// instance owns one Aether child process.
type instance struct {
	cmd         *exec.Cmd
	fingerprint string
	settings    Settings
	name        string
	ready       bool
	lastErr     string
}

// Manager owns one Aether child process per profile, keyed by profile ID. Each
// inbound dials the loopback SOCKS listener of the profile it routes through.
type Manager struct {
	mu        sync.Mutex
	instances map[string]*instance
}

var (
	instanceMgr *Manager
	once        sync.Once
)

// GetManager returns the process-wide Aether manager.
func GetManager() *Manager {
	once.Do(func() { instanceMgr = &Manager{instances: make(map[string]*instance)} })
	return instanceMgr
}

// DialPort returns the loopback port the panel's outbound should dial for the
// given profile, or 0 when the profile is unknown.
func (m *Manager) DialPort(profileID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if in, ok := m.instances[profileID]; ok {
		return in.settings.DialPort()
	}
	return 0
}

// Statuses returns a snapshot of every known profile, running or not.
func (m *Manager) Statuses() []ProfileStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ProfileStatus, 0, len(m.instances))
	for id, in := range m.instances {
		st := ProfileStatus{
			ID:       id,
			Name:     in.name,
			Running:  in.cmd != nil && in.cmd.Process != nil,
			Ready:    in.ready,
			Port:     in.settings.Port(),
			DialPort: in.settings.DialPort(),
			LastErr:  in.lastErr,
			Settings: in.settings,
		}
		if st.Running {
			st.Pid = in.cmd.Process.Pid
		}
		out = append(out, st)
	}
	return out
}

// Reconcile brings the running cores in line with the desired profiles: it
// starts the enabled ones, stops the removed or disabled ones, and restarts a
// core whose arguments changed. Profiles must already be Resolve()d so their
// bind ports are pinned. Errors are logged, not fatal: a core that will not
// start must not break the others or the generated config.
func (m *Manager) Reconcile(profiles []Profile) {
	m.mu.Lock()
	defer m.mu.Unlock()

	desired := make(map[string]Profile, len(profiles))
	for _, p := range profiles {
		desired[p.ID] = p
	}

	// Stop and drop cores no longer desired or now disabled.
	for id, in := range m.instances {
		p, ok := desired[id]
		if !ok || !p.Settings.Enabled {
			m.stopLocked(in)
			delete(m.instances, id)
		}
	}

	for _, p := range profiles {
		if !p.Settings.Enabled {
			continue
		}
		m.ensureLocked(p)
	}
}

// StopAll terminates every running core.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, in := range m.instances {
		m.stopLocked(in)
		delete(m.instances, id)
	}
}

func (m *Manager) ensureLocked(p Profile) {
	fp := p.Settings.Fingerprint()
	if in, ok := m.instances[p.ID]; ok {
		in.name = p.Name
		if in.cmd != nil && in.fingerprint == fp {
			in.settings = p.Settings
			return
		}
		m.stopLocked(in)
	}

	in := &instance{settings: p.Settings, name: p.Name}
	m.instances[p.ID] = in
	if err := m.startLocked(p.ID, in, p.Settings); err != nil {
		logger.Warningf("aether: profile %q (%s): %v", p.Name, p.ID, err)
	}
}

func (m *Manager) startLocked(id string, in *instance, s Settings) error {
	binPath := BinaryPath()
	if _, err := os.Stat(binPath); err != nil {
		in.lastErr = fmt.Sprintf("aether binary not found at %s", binPath)
		return fmt.Errorf("%s", in.lastErr)
	}

	cmd := exec.CommandContext(context.Background(), binPath, s.Args()...)

	// Aether runs psiphon-tunnel-core itself by name; make the bin folder
	// discoverable and run from it so the core finds pt/ beside its binary.
	binDir, _ := filepath.Abs(config.GetBinFolderPath())
	cmd.Env = append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	cmd.Dir = binDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		in.lastErr = err.Error()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		in.lastErr = err.Error()
		return err
	}
	if err := cmd.Start(); err != nil {
		in.lastErr = err.Error()
		return err
	}

	in.cmd = cmd
	in.fingerprint = s.Fingerprint()
	in.ready = false
	in.lastErr = ""

	tag := in.name
	go relayLog(stdout, tag)
	go relayLog(stderr, tag)

	watched := cmd
	go func() {
		err := watched.Wait()
		m.mu.Lock()
		defer m.mu.Unlock()
		if cur, ok := m.instances[id]; ok && cur.cmd == watched {
			if err != nil {
				cur.lastErr = fmt.Sprintf("aether exited: %v", err)
				logger.Warningf("aether: profile %q exited: %v", tag, err)
			}
			cur.cmd = nil
			cur.ready = false
		}
	}()

	logger.Infof("aether: profile %q started on %s:%d (mode=%s psiphon=%s tor=%s)", tag, Loopback, s.Port(), nonEmpty(string(s.Mode), "masque"), nonEmpty(string(s.Psiphon), "off"), nonEmpty(string(s.Tor), "off"))

	port := s.DialPort()
	fp := s.Fingerprint()
	go func() {
		if waitForListener(port, readyTimeout) {
			m.mu.Lock()
			if cur, ok := m.instances[id]; ok && cur.cmd != nil && cur.fingerprint == fp {
				cur.ready = true
			}
			m.mu.Unlock()
		}
	}()
	return nil
}

func (m *Manager) stopLocked(in *instance) {
	if in == nil || in.cmd == nil || in.cmd.Process == nil {
		if in != nil {
			in.cmd = nil
			in.ready = false
		}
		return
	}
	proc := in.cmd.Process
	in.cmd = nil
	in.ready = false
	go func() {
		_ = proc.Signal(os.Interrupt)
		time.Sleep(2 * time.Second)
		_ = proc.Kill()
	}()
}

func relayLog(r io.Reader, tag string) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		logger.Debugf("aether[%s]: %s", tag, scanner.Text())
	}
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
