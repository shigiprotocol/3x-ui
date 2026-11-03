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
			name: "masque default binds loopback socks only",
			in:   Settings{Enabled: true, SocksPort: 2019, Mode: ModeMasque},
			want: []string{"--bind", "127.0.0.1:2019"},
		},
		{
			name: "gool adds --gool",
			in:   Settings{Enabled: true, SocksPort: 3000, Mode: ModeGool},
			want: []string{"--bind", "127.0.0.1:3000", "--gool"},
		},
		{
			name: "mim adds --mim",
			in:   Settings{Enabled: true, SocksPort: 3000, Mode: ModeMim},
			want: []string{"--bind", "127.0.0.1:3000", "--mim"},
		},
		{
			name: "psiphon chain with region and cdn sets",
			in: Settings{
				Enabled: true, SocksPort: 2019, Mode: ModeMasque,
				Psiphon: PsiphonChain, PsiphonRegion: "de", PsiphonCdnSets: "psiphon-akamai,vercel",
			},
			want: []string{"--bind", "127.0.0.1:2019", "--psiphon", "--psiphon-region", "DE", "--psiphon-cdn-sets", "psiphon-akamai,vercel"},
		},
		{
			name: "psiphon only",
			in:   Settings{Enabled: true, SocksPort: 2019, Psiphon: PsiphonOnly},
			want: []string{"--bind", "127.0.0.1:2019", "--psiphon-only"},
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
