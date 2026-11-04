package service

import (
	"encoding/json"
	"fmt"

	"github.com/mhsanaei/3x-ui/v3/internal/aether"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// inboundAetherChoice reads how an inbound opts into Aether: a profile ID in
// aetherProfile, or the legacy routeThroughAether boolean (which means "the first
// profile"). Returns the chosen profile ID resolved against the profile list, or
// "" when the inbound does not route through Aether.
func inboundAetherChoice(inbound *model.Inbound, profiles []aether.Profile) string {
	var parsed struct {
		RouteThroughAether bool   `json:"routeThroughAether"`
		AetherProfile      string `json:"aetherProfile"`
	}
	if err := json.Unmarshal([]byte(inbound.Settings), &parsed); err != nil {
		return ""
	}
	if parsed.AetherProfile != "" {
		for _, p := range profiles {
			if p.ID == parsed.AetherProfile {
				return p.ID
			}
		}
		return ""
	}
	if parsed.RouteThroughAether && len(profiles) > 0 {
		return profiles[0].ID
	}
	return ""
}

// applyAetherCores reconciles the Aether child processes with the saved profiles.
// A profile runs only when it is enabled AND at least one inbound routes through
// it, so an unused profile is not left running. Failures are logged, not fatal.
func (s *XrayService) applyAetherCores(inbounds []*model.Inbound, profiles []aether.Profile) {
	used := make(map[string]bool)
	for _, inbound := range inbounds {
		if inbound == nil || !inbound.Enable || inbound.NodeID != nil || inbound.Tag == "" {
			continue
		}
		if id := inboundAetherChoice(inbound, profiles); id != "" {
			used[id] = true
		}
	}

	resolved := aether.Resolve(profiles)
	for i := range resolved {
		resolved[i].Settings.Enabled = resolved[i].Settings.Enabled && used[resolved[i].ID]
	}
	aether.GetManager().Reconcile(resolved)
}

// injectAetherEgress wires every opted-in inbound through its chosen Aether
// profile: one socks outbound per used profile (dialing that profile's core) and
// a routing rule per inbound to its profile's outbound. Additive and
// hot-appliable; it never touches the stored template.
func injectAetherEgress(cfg *xray.Config, inbounds []*model.Inbound, profiles []aether.Profile) {
	resolved := aether.Resolve(profiles)
	byID := make(map[string]aether.Profile, len(resolved))
	for _, p := range resolved {
		byID[p.ID] = p
	}

	type routed struct {
		inboundTag string
		profileID  string
	}
	var routes []routed
	usedProfiles := make(map[string]bool)
	for _, inbound := range inbounds {
		if inbound == nil || !inbound.Enable || inbound.NodeID != nil || inbound.Tag == "" {
			continue
		}
		id := inboundAetherChoice(inbound, profiles)
		if id == "" {
			continue
		}
		if _, ok := byID[id]; !ok {
			continue
		}
		routes = append(routes, routed{inboundTag: inbound.Tag, profileID: id})
		usedProfiles[id] = true
	}
	if len(routes) == 0 {
		return
	}

	for id := range usedProfiles {
		tag := aether.OutboundTag(id)
		if outboundTagExists(cfg.OutboundConfigs, tag) {
			continue
		}
		if err := appendAetherOutbound(cfg, tag, byID[id].Settings.DialPort()); err != nil {
			logger.Warning("aether egress: failed to append outbound for profile", id, ":", err)
		}
	}

	routing := map[string]any{}
	if len(cfg.RouterConfig) > 0 {
		if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
			logger.Warning("aether egress: routing section is unparsable, skipping rules:", err)
			return
		}
	}
	rules, _ := routing["rules"].([]any)
	prepended := make([]any, 0, len(routes))
	for _, r := range routes {
		prepended = append(prepended, map[string]any{
			"type":        "field",
			"inboundTag":  []any{r.inboundTag},
			"outboundTag": aether.OutboundTag(r.profileID),
		})
	}
	routing["rules"] = append(prepended, rules...)
	newRouting, err := json.Marshal(routing)
	if err != nil {
		logger.Warning("aether egress: failed to rebuild routing section, skipping rules:", err)
		return
	}
	cfg.RouterConfig = json_util.RawMessage(newRouting)
}

// appendAetherOutbound adds a socks outbound with the given tag dialing a core.
func appendAetherOutbound(cfg *xray.Config, tag string, port int) error {
	outbound := map[string]any{
		"tag":      tag,
		"protocol": "socks",
		"settings": map[string]any{
			"servers": []any{
				map[string]any{"address": aether.Loopback, "port": port},
			},
		},
	}

	var existing []any
	if len(cfg.OutboundConfigs) > 0 {
		if err := json.Unmarshal(cfg.OutboundConfigs, &existing); err != nil {
			return fmt.Errorf("parse outbounds: %w", err)
		}
	}
	// Append rather than prepend: a prepended outbound would become Xray's
	// default, silently sending all unrouted traffic through Aether.
	existing = append(existing, outbound)
	combined, err := json.Marshal(existing)
	if err != nil {
		return fmt.Errorf("marshal outbounds: %w", err)
	}
	cfg.OutboundConfigs = json_util.RawMessage(combined)
	return nil
}
