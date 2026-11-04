package aether

import "testing"

// Resolve must pin each profile's bind port to its slot so running cores never
// share a loopback port, and the aux (psiphon/tor) ports must stay inside the
// profile's stride.
func TestResolveAssignsNonOverlappingPorts(t *testing.T) {
	profiles := []Profile{
		{ID: "a", Name: "A", Settings: Settings{Enabled: true, Mode: ModeMasque}},
		{ID: "b", Name: "B", Settings: Settings{Enabled: true, Mode: ModeMasque, Psiphon: PsiphonChain}},
		{ID: "c", Name: "C", Settings: Settings{Enabled: true, Mode: ModeMasque}},
	}
	got := Resolve(profiles)

	wantBase := []int{2019, 2029, 2039}
	for i := range got {
		if got[i].Settings.Port() != wantBase[i] {
			t.Fatalf("profile %d bind port = %d, want %d", i, got[i].Settings.Port(), wantBase[i])
		}
	}
	// B uses psiphon chain: its dial port is the psiphon-bind (base+1), still
	// inside its own stride and not colliding with C's base.
	if dp := got[1].Settings.DialPort(); dp != 2030 {
		t.Fatalf("profile B dial port = %d, want 2030", dp)
	}
	if got[2].Settings.Port() == got[1].Settings.DialPort() {
		t.Fatal("profile C bind port collides with profile B psiphon port")
	}
}

func TestValidateProfilesRejectsDuplicateAndBadIDs(t *testing.T) {
	good := Settings{Mode: ModeMasque}
	tests := []struct {
		name string
		in   []Profile
	}{
		{"duplicate id", []Profile{{ID: "x", Name: "A", Settings: good}, {ID: "x", Name: "B", Settings: good}}},
		{"empty id", []Profile{{ID: "", Name: "A", Settings: good}}},
		{"id with slash", []Profile{{ID: "a/b", Name: "A", Settings: good}}},
		{"empty name", []Profile{{ID: "a", Name: "", Settings: good}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateProfiles(tt.in); err == nil {
				t.Fatalf("ValidateProfiles accepted invalid %+v", tt.in)
			}
		})
	}
}

func TestValidateProfilesAcceptsCleanList(t *testing.T) {
	in := []Profile{
		{ID: "home", Name: "Home", Settings: Settings{Mode: ModeGool, Psiphon: PsiphonChain, PsiphonRegion: "DE"}},
		{ID: "work", Name: "Work", Settings: Settings{Mode: ModeMasque}},
	}
	if err := ValidateProfiles(in); err != nil {
		t.Fatalf("ValidateProfiles rejected a clean list: %v", err)
	}
}

func TestOutboundTag(t *testing.T) {
	if got := OutboundTag("home"); got != "aether-out-home" {
		t.Fatalf("OutboundTag = %q, want aether-out-home", got)
	}
}
