package lumen

import (
	"strings"
	"testing"

	"desktop/internal/control/dto"
	controlv1 "desktop/internal/lumen/controlv1"
)

func TestValidateControlStatusMatchesPinnedRelease(t *testing.T) {
	status := &controlv1.StatusSnapshot{
		Phase: controlv1.Phase_PHASE_DOWNLOADING, Version: "0.1.1", Profile: "metal",
	}
	if err := validateControlStatus(status, "v0.1.1", "darwin-arm64-metal"); err != nil {
		t.Fatalf("validate status: %v", err)
	}
	status.Profile = "cpu"
	if err := validateControlStatus(status, "v0.1.1", "darwin-arm64-metal"); err == nil {
		t.Fatal("backend mismatch was accepted")
	}
	status.Profile = "metal"
	status.Phase = controlv1.Phase_PHASE_FAILED
	status.Error = "weights unavailable"
	if err := validateControlStatus(status, "v0.1.1", "darwin-arm64-metal"); err == nil || !strings.Contains(err.Error(), "weights unavailable") {
		t.Fatalf("failed status error = %v", err)
	}
}

func TestControlStatusDTOCarriesProgressServicesAndFailure(t *testing.T) {
	status := &controlv1.StatusSnapshot{
		Phase: controlv1.Phase_PHASE_DOWNLOADING, Version: "0.1.1", Profile: "metal",
		StartedAtUnixMs: 1234, Seq: 9,
		Download: &controlv1.DownloadProgress{
			Model: "bioclip", File: "vision.bpk", BytesDone: 25, BytesTotal: 100, FilesDone: 1, FilesTotal: 4,
		},
		Services: []*controlv1.ServiceState{
			{Service: "siglip", Phase: controlv1.Phase_PHASE_READY},
			{Service: "bioclip", Phase: controlv1.Phase_PHASE_FAILED, Error: "catalog unavailable"},
		},
	}
	result := controlStatusDTO(status)
	if !result.Connected || result.InferenceReady || result.Phase != dto.LumenControlDownloading {
		t.Fatalf("control status = %+v", result)
	}
	if result.Download == nil || result.Download.BytesDone != 25 || result.Download.FilesTotal != 4 {
		t.Fatalf("download = %+v", result.Download)
	}
	if len(result.Services) != 2 || result.Services[1].Error == nil || result.Services[1].Error.Message != "catalog unavailable" {
		t.Fatalf("services = %+v", result.Services)
	}

	status.Phase = controlv1.Phase_PHASE_READY
	status.Download = nil
	result = controlStatusDTO(status)
	if !result.InferenceReady || result.Phase != dto.LumenControlReady || result.Download != nil {
		t.Fatalf("ready status = %+v", result)
	}
}
