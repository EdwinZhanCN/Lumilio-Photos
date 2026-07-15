package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"

	"server/config"
)

func TestBuildSemanticTextEmbedRequestIncludesResolvedService(t *testing.T) {
	req := buildSemanticTextEmbedRequest([]byte("a photo"), types.ServiceSigLIP)

	if got := req.Meta[types.MetaService]; got != types.ServiceSigLIP {
		t.Fatalf("semantic text request service = %q, want %q", got, types.ServiceSigLIP)
	}
	if req.Task != types.TaskSemanticTextEmbed {
		t.Fatalf("semantic text request task = %q, want %q", req.Task, types.TaskSemanticTextEmbed)
	}
}

func TestNewLumenServiceFromAppConfigDisabled(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.LumenConfig
	}{{"discovery off", config.LumenConfig{DiscoveryEnabled: false, DiscoveryMDNSEnabled: true}}}
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
		DiscoveryEnabled: true, DiscoveryMDNSEnabled: false, DiscoveryHubURL: " http://gw:5866 ",
		DiscoveryStaticNodes: []string{"10.0.0.5:50051"}, DiscoveryServiceType: "_test._tcp", DiscoveryDomain: "example",
		DeploymentID: "manifest-deployment", ResolveTimeout: time.Second, ConnectTimeout: 2 * time.Second,
		RediscoveryBackoffMin: 3 * time.Second, RediscoveryBackoffMax: 4 * time.Second, ScanInterval: 5 * time.Second,
		ChunkAuto: true, ChunkThresholdBytes: 1000, ChunkMaxBytes: 250,
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
	if sdkCfg.Discovery.BrokerURL != "http://gw:5866" {
		t.Fatalf("Discovery.BrokerURL = %q, want trimmed app value", sdkCfg.Discovery.BrokerURL)
	}
	if sdkCfg.Discovery.ServiceType != "_test._tcp" || sdkCfg.Discovery.Domain != "example" || sdkCfg.Discovery.DeploymentID != "manifest-deployment" || sdkCfg.Discovery.ResolveTimeout != time.Second || sdkCfg.Discovery.ConnectTimeout != 2*time.Second || sdkCfg.Discovery.RediscoveryBackoffMin != 3*time.Second || sdkCfg.Discovery.RediscoveryBackoffMax != 4*time.Second || sdkCfg.Discovery.ScanInterval != 5*time.Second {
		t.Fatalf("discovery fields did not map exactly: %+v", sdkCfg.Discovery)
	}
	if !sdkCfg.Chunk.EnableAuto || sdkCfg.Chunk.Threshold != 1000 || sdkCfg.Chunk.MaxChunkBytes != 250 {
		t.Fatalf("chunk fields did not map exactly: %+v", sdkCfg.Chunk)
	}
}

func TestBuildLumenSDKConfigIgnoresAmbientSDKEnv(t *testing.T) {
	t.Setenv("LUMEN_DISCOVERY_DEPLOYMENT_ID", "unit-test-deployment")
	t.Setenv("LUMEN_DISCOVERY_MDNS_ENABLED", "false")

	sdkCfg, err := buildLumenSDKConfig(config.LumenConfig{
		DiscoveryEnabled: true, DiscoveryMDNSEnabled: true, DeploymentID: "manifest-deployment",
	})
	if err != nil {
		t.Fatalf("buildLumenSDKConfig: %v", err)
	}
	if sdkCfg.Discovery.DeploymentID != "manifest-deployment" {
		t.Fatalf("DeploymentID = %q, ambient env changed manifest value", sdkCfg.Discovery.DeploymentID)
	}
	if !sdkCfg.Discovery.MDNSEnabled {
		t.Fatal("app-owned MDNSEnabled must override the SDK env value")
	}
}
