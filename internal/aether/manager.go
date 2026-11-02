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

// readyTimeout bounds how long Start waits for the SOCKS listener to accept a
// connection before reporting the process as not-yet-ready; the process keeps
// running regardless, because Aether may still be scanning for an endpoint.
const readyTimeout = 20 * time.Second

// Status is a snapshot of the manager the panel renders.
type Status struct {
	Running  bool     `json:"running"`
	Ready    bool     `json:"ready"`
	Port     int      `json:"port"`
	Pid      int      `json:"pid"`
	LastErr  string   `json:"lastErr"`
	Settings Settings `json:"settings"`
}

// Manager owns the single Aether child process. One process serves every inbound
// that routes through Aether, because all of them dial the same loopback SOCKS
// listener — the same one-core design PattNG uses on the client.
type Manager struct {
	mu          sync.Mutex
	cmd         *exec.Cmd
	fingerprint string
	settings    Settings
	ready       bool
	lastErr     string
}

var (
	instance *Manager
	once     sync.Once
)

// GetManager returns the process-wide Aether manager.
func GetManager() *Manager {
	once.Do(func() { instance = &Manager{settings: DefaultSettings()} })
	return instance
}

// Status returns a snapshot for the panel.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := Status{
		Running:  m.cmd != nil && m.cmd.Process != nil,
		Ready:    m.ready,
		Port:     m.settings.Port(),
		LastErr:  m.lastErr,
		Settings: m.settings,
	}
	if st.Running {
		st.Pid = m.cmd.Process.Pid
	}
	return st
}

// IsRunning reports whether the child process is alive.
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cmd != nil && m.cmd.Process != nil
}

// Port is the loopback SOCKS port the current settings use.
func (m *Manager) Port() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings.Port()
}

// Apply reconciles the running process with desired settings: starts it when it
// should run and does not, stops it when it should not, and restarts it when the
// arguments changed. A no-op when already matching, so it is safe to call on
// every config rebuild.
func (m *Manager) Apply(s Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if !s.Enabled {
		m.stopLocked()
		m.settings = s
		return nil
	}

	if m.cmd != nil && m.fingerprint == s.Fingerprint() {
		m.settings = s
		return nil
	}

	m.stopLocked()
	m.settings = s
	return m.startLocked(s)
}

// Stop terminates the process if running.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()
}

func (m *Manager) startLocked(s Settings) error {
	binPath := BinaryPath()
	if _, err := os.Stat(binPath); err != nil {
		m.lastErr = fmt.Sprintf("aether binary not found at %s", binPath)
		return fmt.Errorf("aether: %s", m.lastErr)
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, binPath, s.Args()...)

	// Aether runs psiphon-tunnel-core itself by name; make the bin folder
	// discoverable, and point its data dir at a writable, persistent place.
	binDir, _ := filepath.Abs(config.GetBinFolderPath())
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	cmd.Dir = binDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.lastErr = err.Error()
		return fmt.Errorf("aether: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.lastErr = err.Error()
		return fmt.Errorf("aether: stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		m.lastErr = err.Error()
		return fmt.Errorf("aether: start: %w", err)
	}

	m.cmd = cmd
	m.fingerprint = s.Fingerprint()
	m.ready = false
	m.lastErr = ""

	go relayLog(stdout)
	go relayLog(stderr)

	// Watch for exit so a crashed process does not look alive in Status.
	watched := cmd
	go func() {
		err := watched.Wait()
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.cmd == watched {
			if err != nil {
				m.lastErr = fmt.Sprintf("aether exited: %v", err)
				logger.Warningf("aether: process exited: %v", err)
			}
			m.cmd = nil
			m.ready = false
		}
	}()

	logger.Infof("aether: started on %s:%d (mode=%s psiphon=%s)", Loopback, s.Port(), nonEmpty(string(s.Mode), "masque"), nonEmpty(string(s.Psiphon), "off"))

	port := s.Port()
	go func() {
		if waitForListener(port, readyTimeout) {
			m.mu.Lock()
			if m.fingerprint == s.Fingerprint() && m.cmd != nil {
				m.ready = true
			}
			m.mu.Unlock()
		}
	}()
	return nil
}

func (m *Manager) stopLocked() {
	if m.cmd == nil || m.cmd.Process == nil {
		m.cmd = nil
		m.ready = false
		return
	}
	proc := m.cmd.Process
	m.cmd = nil
	m.ready = false
	// Signal then hard-kill after a grace period, off the lock-holding path.
	go func() {
		_ = proc.Signal(os.Interrupt)
		time.Sleep(2 * time.Second)
		_ = proc.Kill()
	}()
}

func relayLog(r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		logger.Debugf("aether: %s", scanner.Text())
	}
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
