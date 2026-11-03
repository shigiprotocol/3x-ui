package aether

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestArgsBuildsBindAndModeFlags(t *testing.T) {
	tests := []struct {
		name string
		in   Settings
		want []string
	}{
		{
			name: "masque default sends explicit --masque and --h3 to skip both menus",
			in:   Settings{Enabled: true, SocksPort: 2019, Mode: ModeMasque, Transport: TransportH3},
			want: []string{"--bind", "127.0.0.1:2019", "--masque", "--h3", "--scan", "balanced", "-4"},
		},
		{
			name: "h2 transport sends --h2",
			in:   Settings{Enabled: true, SocksPort: 2019, Mode: ModeMasque, Transport: TransportH2},
			want: []string{"--bind", "127.0.0.1:2019", "--masque", "--h2", "--scan", "balanced", "-4"},
		},
		{
			name: "gool adds --gool",
			in:   Settings{Enabled: true, SocksPort: 3000, Mode: ModeGool, Transport: TransportH3},
			want: []string{"--bind", "127.0.0.1:3000", "--gool", "--h3", "--scan", "balanced", "-4"},
		},
		{
			name: "mim adds --mim",
			in:   Settings{Enabled: true, SocksPort: 3000, Mode: ModeMim, Transport: TransportH3},
			want: []string{"--bind", "127.0.0.1:3000", "--mim", "--h3", "--scan", "balanced", "-4"},
		},
		{
			name: "psiphon chain with region and cdn sets",
			in: Settings{
				Enabled: true, SocksPort: 2019, Mode: ModeMasque, Transport: TransportH3,
				Psiphon: PsiphonChain, PsiphonRegion: "de", PsiphonCdnSets: "psiphon-akamai,vercel",
			},
			want: []string{"--bind", "127.0.0.1:2019", "--masque", "--h3", "--scan", "balanced", "-4", "--psiphon", "--psiphon-region", "DE", "--psiphon-cdn-sets", "psiphon-akamai,vercel"},
		},
		{
			name: "psiphon only still names the base protocol",
			in:   Settings{Enabled: true, SocksPort: 2019, Mode: ModeMasque, Transport: TransportH3, Psiphon: PsiphonOnly},
			want: []string{"--bind", "127.0.0.1:2019", "--masque", "--h3", "--scan", "balanced", "-4", "--psiphon-only"},
		},
		{
			name: "wireguard omits the masque transport flag",
			in:   Settings{Enabled: true, SocksPort: 2019, Mode: ModeWireGuard, Transport: TransportH3, Scan: ScanTurbo},
			want: []string{"--bind", "127.0.0.1:2019", "--wg", "--scan", "turbo", "-4"},
		},
		{
			name: "gool-classic omits the masque transport flag",
			in:   Settings{Enabled: true, SocksPort: 2019, Mode: ModeGoolClassic, Transport: TransportH2, Scan: ScanBalanced},
			want: []string{"--bind", "127.0.0.1:2019", "--gool-classic", "--scan", "balanced", "-4"},
		},
		{
			name: "h2 with fragment adds --fragment",
			in:   Settings{Enabled: true, SocksPort: 2019, Mode: ModeMasque, Transport: TransportH2, Fragment: true, Scan: ScanBalanced},
			want: []string{"--bind", "127.0.0.1:2019", "--masque", "--h2", "--fragment", "--scan", "balanced", "-4"},
		},
		{
			name: "ipv6 and noize and thorough scan",
			in:   Settings{Enabled: true, SocksPort: 2019, Mode: ModeMasque, Transport: TransportH3, Scan: ScanThorough, IPVersion: IPv6, Obfuscation: NoizeGfw},
			want: []string{"--bind", "127.0.0.1:2019", "--masque", "--h3", "--scan", "thorough", "-6", "--noize", "gfw"},
		},
		{
			name: "dual ip",
			in:   Settings{Enabled: true, SocksPort: 2019, Mode: ModeMasque, Transport: TransportH3, Scan: ScanBalanced, IPVersion: IPAny},
			want: []string{"--bind", "127.0.0.1:2019", "--masque", "--h3", "--scan", "balanced", "--dual"},
		},
		{
			name: "tor chain",
			in:   Settings{Enabled: true, SocksPort: 2019, Mode: ModeMasque, Transport: TransportH3, Scan: ScanBalanced, Tor: TorChain},
			want: []string{"--bind", "127.0.0.1:2019", "--masque", "--h3", "--scan", "balanced", "-4", "--tor"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in.Args()
			if !slices.Equal(got, tt.want) {
				t.Fatalf("Args() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Region and cdn-set fields reach a command line, so an injected token must be
// rejected rather than silently appended as a flag.
func TestValidateRejectsFlagInjection(t *testing.T) {
	tests := []struct {
		name string
		in   Settings
	}{
		{"region with flag", Settings{Mode: ModeMasque, Psiphon: PsiphonChain, PsiphonRegion: "--psiphon-bin=/x"}},
		{"region too long", Settings{Mode: ModeMasque, Psiphon: PsiphonChain, PsiphonRegion: "DEU"}},
		{"cdn sets with space flag", Settings{Mode: ModeMasque, Psiphon: PsiphonChain, PsiphonCdnSets: "a --tor"}},
		{"unknown mode", Settings{Mode: Mode("evil")}},
		{"unknown psiphon", Settings{Mode: ModeMasque, Psiphon: PsiphonMode("evil")}},
		{"unknown scan", Settings{Mode: ModeMasque, Scan: ScanMode("evil")}},
		{"unknown ip version", Settings{Mode: ModeMasque, IPVersion: IPVersion("evil")}},
		{"unknown obfuscation", Settings{Mode: ModeMasque, Obfuscation: Obfuscation("evil")}},
		{"unknown tor", Settings{Mode: ModeMasque, Tor: TorMode("evil")}},
		{"tor and psiphon both on", Settings{Mode: ModeMasque, Psiphon: PsiphonChain, Tor: TorChain}},
		{"port out of range", Settings{Mode: ModeMasque, SocksPort: 70000}},
		{"extra arg with newline", Settings{Mode: ModeMasque, ExtraArgs: []string{"ok\ninjected"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.in.Validate(); err == nil {
				t.Fatalf("Validate() accepted invalid settings %+v", tt.in)
			}
		})
	}
}

func TestValidateAcceptsCleanSettings(t *testing.T) {
	s := Settings{Enabled: true, SocksPort: 2019, Mode: ModeGool, Psiphon: PsiphonChain, PsiphonRegion: "US", PsiphonCdnSets: "vercel,github"}
	if err := s.Validate(); err != nil {
		t.Fatalf("Validate() rejected clean settings: %v", err)
	}
}

// A settings row saved with a zero port and blank mode (an early/partial save)
// must come back with the effective defaults, or the settings page shows 0 and
// an empty mode dropdown.
func TestNormalizedFillsZeroValues(t *testing.T) {
	got := Settings{Enabled: true, SocksPort: 0, Mode: ""}.Normalized()
	if got.SocksPort != DefaultSocksPort {
		t.Fatalf("SocksPort = %d, want default %d", got.SocksPort, DefaultSocksPort)
	}
	if got.Mode != ModeMasque {
		t.Fatalf("Mode = %q, want %q", got.Mode, ModeMasque)
	}
	if got.ExtraArgs == nil {
		t.Fatal("ExtraArgs is nil, want empty slice")
	}
}

func TestNormalizedKeepsRealValues(t *testing.T) {
	got := Settings{Enabled: true, SocksPort: 3000, Mode: ModeGool}.Normalized()
	if got.SocksPort != 3000 || got.Mode != ModeGool {
		t.Fatalf("Normalized changed real values: port=%d mode=%q", got.SocksPort, got.Mode)
	}
}

// A relative program path with no slash makes exec search PATH instead of the
// bin folder, so the core fails to launch ("fork/exec bin/aether: no such file
// or directory"). BinaryPath must be absolute.
func TestBinaryPathIsAbsolute(t *testing.T) {
	if p := BinaryPath(); !filepath.IsAbs(p) {
		t.Fatalf("BinaryPath() = %q, want an absolute path", p)
	}
}

func TestPortAppliesDefault(t *testing.T) {
	if got := (Settings{}).Port(); got != DefaultSocksPort {
		t.Fatalf("Port() with zero = %d, want default %d", got, DefaultSocksPort)
	}
	if got := (Settings{SocksPort: 4000}).Port(); got != 4000 {
		t.Fatalf("Port() = %d, want 4000", got)
	}
}

// A changed argument must change the fingerprint, or the manager would not
// restart the process when settings change; an unrelated equal config must not.
func TestFingerprintTracksArgs(t *testing.T) {
	a := Settings{Enabled: true, SocksPort: 2019, Mode: ModeMasque}
	b := Settings{Enabled: true, SocksPort: 2019, Mode: ModeGool}
	if a.Fingerprint() == b.Fingerprint() {
		t.Fatal("fingerprint did not change when mode changed")
	}
	if a.Fingerprint() != (Settings{Enabled: true, SocksPort: 2019, Mode: ModeMasque}).Fingerprint() {
		t.Fatal("fingerprint changed for equal settings")
	}
	if off := (Settings{Enabled: false}).Fingerprint(); off != "off" {
		t.Fatalf("disabled fingerprint = %q, want off", off)
	}
}

func TestArgsNeverContainsControlChars(t *testing.T) {
	s := Settings{Enabled: true, SocksPort: 2019, Mode: ModeMasque, Psiphon: PsiphonChain, PsiphonRegion: "FR"}
	for _, a := range s.Args() {
		if strings.ContainsAny(a, "\n\r\x00") {
			t.Fatalf("arg %q contains a control character", a)
		}
	}
}
