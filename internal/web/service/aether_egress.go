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

// AetherOutboundTag is the tag of the socks outbound injected into the generated
// config when any inbound opts into Aether. Every opted-in inbound routes to it,
// and it dials the one Aether core process the panel runs on loopback — the same
// one-core design PattNG uses on the client.
const AetherOutboundTag = "aether-out"

// applyAetherCore reconciles the Aether child process with the saved settings.
// It starts the core when at least one enabled inbound opts in and the feature is
// enabled, and stops it otherwise, so the process is not left running when no
// inbound uses it. A start failure is logged, not fatal: the generated config is
// still valid and the core can come up on a later rebuild.
func (s *XrayService) applyAetherCore(inbounds []*model.Inbound) {
	cfg, err := s.settingService.GetAetherSettings()
	if err != nil {
		logger.Warning("aether: read settings failed:", err)
		return
	}

	anyOptedIn := false
	for _, inbound := range inbounds {
		if inbound != nil && inbound.Enable && inbound.NodeID == nil && inbound.Tag != "" && inboundRoutesThroughAether(inbound) {
			anyOptedIn = true
			break
		}
	}
	// Run the core only when it is both enabled and actually used.
	cfg.Enabled = cfg.Enabled && anyOptedIn

	if err := aether.GetManager().Apply(cfg); err != nil {
		logger.Warning("aether: apply core settings failed:", err)
	}
}

// inboundRoutesThroughAether reports whether an inbound asked for its traffic to
// egress through Aether. The opt-in is a boolean in the inbound's settings JSON,
// so no schema migration is needed, matching how mtproto stores routeThroughXray.
func inboundRoutesThroughAether(inbound *model.Inbound) bool {
	var parsed struct {
		RouteThroughAether bool `json:"routeThroughAether"`
	}
	if err := json.Unmarshal([]byte(inbound.Settings), &parsed); err != nil {
		return false
	}
	return parsed.RouteThroughAether
}

// injectAetherEgress wires every opted-in inbound through the Aether core. It is
// a no-op unless at least one enabled, local inbound opts in. When some do, it
// appends one socks outbound dialing the Aether manager's loopback SOCKS port and
// prepends a routing rule per inbound sending that inbound's traffic to it.
//
// The generated state is additive and hot-appliable; it never touches the stored
// template. If the Aether core is not running the outbound still resolves (socks
// dial to a dead port simply fails per-connection), so routing stays valid and
// the core can come up later without a config rebuild.
func injectAetherEgress(cfg *xray.Config, inbounds []*model.Inbound) {
	var opted []*model.Inbound
	for _, inbound := range inbounds {
		if inbound == nil || !inbound.Enable || inbound.NodeID != nil {
			continue
		}
		if inbound.Tag == "" {
			continue
		}
		if inboundRoutesThroughAether(inbound) {
			opted = append(opted, inbound)
		}
	}
	if len(opted) == 0 {
		return
	}

	port := aether.GetManager().Port()

	if !outboundTagExists(cfg.OutboundConfigs, AetherOutboundTag) {
		if err := appendAetherOutbound(cfg, port); err != nil {
			logger.Warning("aether egress: failed to append outbound, skipping injection:", err)
			return
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
	prepended := make([]any, 0, len(opted))
	for _, inbound := range opted {
		prepended = append(prepended, map[string]any{
			"type":        "field",
			"inboundTag":  []any{inbound.Tag},
			"outboundTag": AetherOutboundTag,
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

// appendAetherOutbound adds the single socks outbound that dials the Aether core.
func appendAetherOutbound(cfg *xray.Config, port int) error {
	outbound := map[string]any{
		"tag":      AetherOutboundTag,
		"protocol": "socks",
		"settings": map[string]any{
			"servers": []any{
				map[string]any{
					"address": aether.Loopback,
					"port":    port,
				},
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
