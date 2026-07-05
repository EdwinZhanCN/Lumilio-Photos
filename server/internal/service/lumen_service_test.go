package service

import (
	"context"
	"errors"
	"testing"

	"server/config"
)

func TestNewLumenServiceFromAppConfigDisabled(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.LumenConfig
	}{
		{"discovery off", config.LumenConfig{DiscoveryEnabled: false, DiscoveryMDNSEnabled: true}},
		{"no backend configured", config.LumenConfig{DiscoveryEnabled: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, err := NewLumenServiceFromAppConfig(tc.cfg, nil)
			if err != nil {
				t.Fatalf("disabled config must not fail boot: %v", err)
			}

			ctx := context.Background()
			if err := svc.Start(ctx); err != nil {
				t.Fatalf("disabled Start: %v", err)
			}
			if svc.IsTaskAvailable("semantic_text_embed") {
				t.Fatal("disabled service must report no available tasks")
			}
			if _, err := svc.SemanticTextEmbed(ctx, []byte("q")); !errors.Is(err, ErrLumenDisabled) {
				t.Fatalf("SemanticTextEmbed error = %v, want ErrLumenDisabled", err)
			}
			if _, err := svc.SemanticImageEmbed(ctx, nil); !errors.Is(err, ErrLumenDisabled) {
				t.Fatalf("SemanticImageEmbed error = %v, want ErrLumenDisabled", err)
			}
			if stats := svc.PoolStats(); stats.TotalConnections != 0 || stats.HealthyConnections != 0 {
				t.Fatalf("disabled pool stats should be zero, got %+v", stats)
			}
			if nodes := svc.GetNodes(); len(nodes) != 0 {
				t.Fatalf("disabled service should report no nodes, got %d", len(nodes))
			}
			if err := svc.Close(); err != nil {
				t.Fatalf("disabled Close: %v", err)
			}
		})
	}
}

func TestBuildLumenSDKConfigMapsAppFields(t *testing.T) {
	sdkCfg, err := buildLumenSDKConfig(config.LumenConfig{
		DiscoveryEnabled:     true,
		DiscoveryMDNSEnabled: false,
		DiscoveryHubURL:      " http://gw:5866 ",
		DiscoveryStaticNodes: []string{" 10.0.0.5:50051 ", ""},
	})
	if err != nil {
		t.Fatalf("buildLumenSDKConfig: %v", err)
	}
	if len(sdkCfg.Discovery.StaticNodes) != 1 || sdkCfg.Discovery.StaticNodes[0] != "10.0.0.5:50051" {
		t.Fatalf("Discovery.StaticNodes = %v, want trimmed [10.0.0.5:50051]", sdkCfg.Discovery.StaticNodes)
	}
	if !sdkCfg.Discovery.Enabled {
		t.Fatal("Discovery.Enabled should map from app config")
	}
	if sdkCfg.Discovery.MDNSEnabled {
		t.Fatal("Discovery.MDNSEnabled should map from app config (false)")
	}
	if sdkCfg.Discovery.HubURL != "http://gw:5866" {
		t.Fatalf("Discovery.HubURL = %q, want trimmed app value", sdkCfg.Discovery.HubURL)
	}
	if sdkCfg.Discovery.ScanInterval <= 0 || sdkCfg.Discovery.ConnectTimeout <= 0 {
		t.Fatalf("SDK defaults should be preserved, got %+v", sdkCfg.Discovery)
	}
}

func TestBuildLumenSDKConfigKeepsSDKEnvKnobs(t *testing.T) {
	t.Setenv("LUMEN_DISCOVERY_DEPLOYMENT_ID", "unit-test-deployment")
	// App-owned fields must win over the same env vars the SDK also reads.
	t.Setenv("LUMEN_DISCOVERY_MDNS_ENABLED", "false")

	sdkCfg, err := buildLumenSDKConfig(config.LumenConfig{
		DiscoveryEnabled:     true,
		DiscoveryMDNSEnabled: true,
	})
	if err != nil {
		t.Fatalf("buildLumenSDKConfig: %v", err)
	}
	if sdkCfg.Discovery.DeploymentID != "unit-test-deployment" {
		t.Fatalf("DeploymentID = %q, want SDK env knob to apply", sdkCfg.Discovery.DeploymentID)
	}
	if !sdkCfg.Discovery.MDNSEnabled {
		t.Fatal("app-owned MDNSEnabled must override the SDK env value")
	}
}
