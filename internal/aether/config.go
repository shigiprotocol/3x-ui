// Package aether runs the Aether censorship-circumvention core as a managed
// child process and exposes its local SOCKS5 listener, so an inbound can route
// its outbound traffic through Aether (and, optionally, Psiphon carried inside
// Aether's tunnel). Aether has no server-side listener of its own: it dials out
// and offers a loopback SOCKS proxy, so the panel wires it in exactly as PattNG
// does on the client side — as a socks outbound every selected inbound routes to.
package aether

import (
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

// BinaryName is the Aether executable looked up in the panel's bin folder.
const BinaryName = "aether"

// PsiphonBinaryName is the psiphon-tunnel-core executable Aether runs itself
// when a Psiphon mode is selected; it must sit in the same bin folder so Aether
// finds it on PATH (the manager adds the bin folder to the child's PATH).
const PsiphonBinaryName = "psiphon-tunnel-core"

// DefaultSocksPort is the loopback SOCKS5 port Aether listens on and the panel's
// injected socks outbound dials. Kept off the common 1080 to avoid clashing with
// a user's own proxy.
const DefaultSocksPort = 2019

// Loopback is the address the SOCKS listener binds and the outbound dials.
const Loopback = "127.0.0.1"

// Mode is the top-level tunnel shape Aether runs in.
type Mode string

const (
	// ModeMasque is Aether's default: a MASQUE (HTTP/3 or HTTP/2) tunnel over
	// discovered endpoints.
	ModeMasque Mode = "masque"
	// ModeGool is WireGuard-over-MASQUE (--gool), a WireGuard tunnel inside a
	// MASQUE one for a different exit address.
	ModeGool Mode = "gool"
	// ModeMim is nested MASQUE (--mim), a masque tunnel inside another one.
	ModeMim Mode = "mim"
)

func (m Mode) valid() bool {
	switch m {
	case ModeMasque, ModeGool, ModeMim, "":
		return true
	default:
		return false
	}
}

// PsiphonMode selects how Psiphon is combined with the Aether tunnel. Psiphon is
// built into the Aether core; the panel only picks the flag.
type PsiphonMode string

const (
	// PsiphonOff runs no Psiphon.
	PsiphonOff PsiphonMode = ""
	// PsiphonChain carries Psiphon inside the tunnel: aether -> warp -> psiphon
	// -> internet (--psiphon).
	PsiphonChain PsiphonMode = "chain"
	// PsiphonReverse dials the tunnel through Psiphon instead (--psiphon-reverse).
	PsiphonReverse PsiphonMode = "reverse"
	// PsiphonOnly skips the Aether tunnel and serves plain Psiphon (--psiphon-only).
	PsiphonOnly PsiphonMode = "only"
)

func (p PsiphonMode) valid() bool {
	switch p {
	case PsiphonOff, PsiphonChain, PsiphonReverse, PsiphonOnly:
		return true
	default:
		return false
	}
}

func (p PsiphonMode) enabled() bool { return p != PsiphonOff }

// Settings is the panel-side Aether configuration persisted as one Setting row
// and turned into the child process's argument list by Args.
type Settings struct {
	Enabled bool `json:"enabled"`
	// SocksPort is the loopback SOCKS5 port; 0 means DefaultSocksPort.
	SocksPort int  `json:"socksPort"`
	Mode      Mode `json:"mode"`

	Psiphon PsiphonMode `json:"psiphon"`
	// PsiphonRegion asks Psiphon to leave from this country, e.g. "DE"; empty lets
	// it pick. Validated as a 2-letter code so it cannot smuggle extra flags.
	PsiphonRegion string `json:"psiphonRegion"`
	// PsiphonCdnSets picks which built-in edge lists Psiphon fronts through,
	// comma-separated (e.g. "psiphon-akamai,vercel"); empty means Psiphon's default.
	PsiphonCdnSets string `json:"psiphonCdnSets"`

	// ExtraArgs are additional raw flags for power users, applied verbatim after
	// the generated ones. Each entry is one argv token (no shell splitting).
	ExtraArgs []string `json:"extraArgs"`
}

// DefaultSettings is the off state.
func DefaultSettings() Settings {
	return Settings{Enabled: false, SocksPort: DefaultSocksPort, Mode: ModeMasque, ExtraArgs: []string{}}
}

// Port is the effective SOCKS port, applying the default.
func (s Settings) Port() int {
	if s.SocksPort <= 0 {
		return DefaultSocksPort
	}
	return s.SocksPort
}

// Normalized fills zero-value fields the UI would otherwise show as 0 / blank
// (a settings row saved before a field had a value, or a partial save) with
// their effective defaults, so the settings page always renders real choices.
func (s Settings) Normalized() Settings {
	s.SocksPort = s.Port()
	if s.Mode == "" {
		s.Mode = ModeMasque
	}
	if s.ExtraArgs == nil {
		s.ExtraArgs = []string{}
	}
	return s
}

// Validate rejects a settings object the manager would refuse to run, naming the
// field so the panel can show a usable message. Free-text fields that reach the
// command line are checked here so a value cannot inject an unintended flag.
func (s Settings) Validate() error {
	if !s.Mode.valid() {
		return fmt.Errorf("aether: unknown mode %q", s.Mode)
	}
	if !s.Psiphon.valid() {
		return fmt.Errorf("aether: unknown psiphon mode %q", s.Psiphon)
	}
	if s.SocksPort < 0 || s.SocksPort > 65535 {
		return fmt.Errorf("aether: socks port %d out of range", s.SocksPort)
	}
	if s.PsiphonRegion != "" && !isCountryCode(s.PsiphonRegion) {
		return fmt.Errorf("aether: psiphon region %q is not a 2-letter country code", s.PsiphonRegion)
	}
	if s.PsiphonCdnSets != "" && !isCdnSetList(s.PsiphonCdnSets) {
		return fmt.Errorf("aether: psiphon cdn sets %q has invalid characters", s.PsiphonCdnSets)
	}
	for _, a := range s.ExtraArgs {
		if strings.ContainsAny(a, "\n\r\x00") {
			return fmt.Errorf("aether: an extra argument contains a control character")
		}
	}
	return nil
}

// Args builds the Aether argument list from the settings. The SOCKS listener is
// pinned to loopback on the chosen port; the mode and Psiphon flags follow.
func (s Settings) Args() []string {
	args := []string{"--bind", net.JoinHostPort(Loopback, strconv.Itoa(s.Port()))}

	switch s.Mode {
	case ModeGool:
		args = append(args, "--gool")
	case ModeMim:
		args = append(args, "--mim")
	}

	switch s.Psiphon {
	case PsiphonChain:
		args = append(args, "--psiphon")
	case PsiphonReverse:
		args = append(args, "--psiphon-reverse")
	case PsiphonOnly:
		args = append(args, "--psiphon-only")
	}
	if s.Psiphon.enabled() {
		if s.PsiphonRegion != "" {
			args = append(args, "--psiphon-region", strings.ToUpper(s.PsiphonRegion))
		}
		if s.PsiphonCdnSets != "" {
			args = append(args, "--psiphon-cdn-sets", s.PsiphonCdnSets)
		}
	}

	args = append(args, s.ExtraArgs...)
	return args
}

// Fingerprint changes whenever a restart is needed: the manager compares it to
// decide whether a running process already matches the desired settings.
func (s Settings) Fingerprint() string {
	if !s.Enabled {
		return "off"
	}
	return strconv.Itoa(s.Port()) + "|" + strings.Join(s.Args(), " ")
}

// BinaryPath is the Aether executable's absolute path in the panel's bin folder.
// It must be absolute: exec with a relative program name that has no slash of
// its own searches PATH, not the bin folder, so "bin/aether" fails to launch.
func BinaryPath() string {
	p := config.GetBinFolderPath() + "/" + BinaryName
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func isCountryCode(s string) bool {
	if len(s) != 2 {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

// isCdnSetList allows letters, digits, dash and comma only, so the value is a
// list of set names and nothing that could become a separate flag.
func isCdnSetList(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == ',':
		default:
			return false
		}
	}
	return true
}
