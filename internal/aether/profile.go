package aether

import (
	"fmt"
	"strings"
)

// profilePortStride is how far apart consecutive profiles' loopback ports sit, so
// each profile's bind / psiphon-bind / tor-bind ports never collide with the
// next profile's.
const profilePortStride = 10

// MaxProfiles caps how many Aether cores the panel runs at once; each is a child
// process with its own tunnel, so the number is kept small and deterministic.
const MaxProfiles = 16

// Profile is one named Aether configuration the panel runs as its own core. An
// inbound routes through a profile by its ID. Several profiles let a server hold
// different Aether configs at once (for example, one pasted from PattNG).
type Profile struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Settings Settings `json:"settings"`
}

// BasePortForIndex is the loopback bind port a profile in the given slot uses.
func BasePortForIndex(index int) int {
	return DefaultSocksPort + index*profilePortStride
}

// resolved returns the profile's settings with its bind port pinned to the slot,
// so the running cores never share a loopback port. Index is the profile's
// position in the saved list.
func (p Profile) resolved(index int) Settings {
	s := p.Settings.Normalized()
	s.SocksPort = BasePortForIndex(index)
	return s
}

// Validate checks the profile is runnable: a non-empty ID and name, and valid
// settings.
func (p Profile) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("aether: a profile is missing its id")
	}
	if strings.ContainsAny(p.ID, " \t\n\r/") {
		return fmt.Errorf("aether: profile id %q has invalid characters", p.ID)
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("aether: profile %q is missing a name", p.ID)
	}
	return p.Settings.Validate()
}

// ValidateProfiles checks a whole list: each profile is valid and IDs are unique.
func ValidateProfiles(profiles []Profile) error {
	if len(profiles) > MaxProfiles {
		return fmt.Errorf("aether: too many profiles (%d), max is %d", len(profiles), MaxProfiles)
	}
	seen := make(map[string]struct{}, len(profiles))
	for _, p := range profiles {
		if err := p.Validate(); err != nil {
			return err
		}
		if _, dup := seen[p.ID]; dup {
			return fmt.Errorf("aether: duplicate profile id %q", p.ID)
		}
		seen[p.ID] = struct{}{}
	}
	return nil
}

// Resolve pins each profile's bind port to its slot and returns the runnable
// copies, in list order.
func Resolve(profiles []Profile) []Profile {
	out := make([]Profile, len(profiles))
	for i, p := range profiles {
		out[i] = Profile{ID: p.ID, Name: p.Name, Settings: p.resolved(i)}
	}
	return out
}

// OutboundTag is the Xray outbound tag for a profile: the socks outbound that
// dials this profile's core.
func OutboundTag(profileID string) string {
	return "aether-out-" + profileID
}
