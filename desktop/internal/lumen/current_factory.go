package lumen

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"desktop/internal/control/dto"
	controlv1 "desktop/internal/lumen/controlv1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type CurrentFactory struct {
	Root             string
	ConfigPath       string
	OwnerLock        string
	Endpoint         string
	SupervisorBinary string
	Prepare          func(ctx context.Context, profile, hubBinary string) error
}

// Start resolves current.json for every generation so a newly installed Hub
// can start without restarting Desktop and an old version is never selected by
// directory scanning or PATH lookup.
func (f CurrentFactory) Start(ctx context.Context, id uint64, profile string) (Process, error) {
	current, err := LoadCurrent(f.Root)
	if err != nil {
		return Process{}, err
	}
	if current.Profile != profile {
		return Process{}, fmt.Errorf("installed Lumen profile %q does not match requested profile %q", current.Profile, profile)
	}
	hubBinary, err := safeJoin(f.Root, current.Binary)
	if err != nil {
		return Process{}, err
	}
	if f.Prepare != nil {
		if err := f.Prepare(ctx, profile, hubBinary); err != nil {
			return Process{}, fmt.Errorf("prepare Lumen configuration: %w", err)
		}
	}
	supervisor := f.SupervisorBinary
	if supervisor == "" {
		supervisor, err = os.Executable()
		if err != nil {
			return Process{}, err
		}
	}
	endpoint := f.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	process, err := (ExecFactory{
		Binary:       supervisor,
		Args:         SupervisorArgs(hubBinary, f.ConfigPath),
		WorkDir:      filepath.Dir(hubBinary),
		OwnerLock:    f.OwnerLock,
		Profile:      profile,
		Endpoint:     endpoint,
		Probe:        controlReadinessProbe(endpoint, current.Version),
		ProbeTimeout: ReadyBudget,
	}).Start(ctx, id, profile)
	if err != nil {
		return Process{}, err
	}
	lifetime := process.Lifetime
	if lifetime == nil {
		lifetime = ctx
	}
	process.Status = watchControlStatus(lifetime, endpoint, current.Version, profile)
	go monitorControlFailure(lifetime, endpoint, current.Version, profile, process.Cancel)
	return process, nil
}

func controlReadinessProbe(endpoint, expectedVersion string) ReadinessProbe {
	return func(ctx context.Context, profile, _ string) error {
		connection, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return err
		}
		defer connection.Close()
		client := controlv1.NewControlClient(connection)
		var lastErr error
		for {
			probeCtx, cancel := context.WithTimeout(ctx, time.Second)
			status, callErr := client.GetStatus(probeCtx, &emptypb.Empty{})
			cancel()
			if callErr == nil {
				if err := validateControlStatus(status, expectedVersion, profile); err != nil {
					return err
				}
				return nil
			}
			lastErr = callErr
			select {
			case <-ctx.Done():
				if lastErr != nil {
					return fmt.Errorf("Lumen control plane unavailable: %w", lastErr)
				}
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
			}
		}
	}
}

func validateControlStatus(status *controlv1.StatusSnapshot, expectedVersion, expectedProfile string) error {
	if err := validateControlIdentity(status, expectedVersion, expectedProfile); err != nil {
		return err
	}
	if status.GetPhase() == controlv1.Phase_PHASE_FAILED {
		return fmt.Errorf("Lumen startup failed: %s", strings.TrimSpace(status.GetError()))
	}
	if status.GetPhase() == controlv1.Phase_PHASE_UNSPECIFIED || status.GetPhase() == controlv1.Phase_PHASE_STOPPING {
		return fmt.Errorf("Lumen control plane reported invalid startup phase %s", status.GetPhase())
	}
	return nil
}

func validateControlIdentity(status *controlv1.StatusSnapshot, expectedVersion, expectedProfile string) error {
	if status == nil {
		return errors.New("Lumen control plane returned no status")
	}
	actualVersion := strings.TrimPrefix(strings.TrimSpace(status.GetVersion()), "v")
	wantVersion := strings.TrimPrefix(strings.TrimSpace(expectedVersion), "v")
	if actualVersion == "" || actualVersion != wantVersion {
		return fmt.Errorf("Lumen control plane version %q does not match installed version %q", status.GetVersion(), expectedVersion)
	}
	wantBackend := releaseProfileBackend(expectedProfile)
	if wantBackend == "" || status.GetProfile() != wantBackend {
		return fmt.Errorf("Lumen control plane backend %q does not match release profile %q", status.GetProfile(), expectedProfile)
	}
	return nil
}

func watchControlStatus(ctx context.Context, endpoint, version, profile string) <-chan dto.LumenControlStatus {
	updates := make(chan dto.LumenControlStatus, 1)
	go func() {
		defer close(updates)
		for ctx.Err() == nil {
			connection, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err == nil {
				stream, streamErr := controlv1.NewControlClient(connection).WatchStatus(ctx, &emptypb.Empty{})
				if streamErr == nil {
					for {
						status, receiveErr := stream.Recv()
						if receiveErr != nil {
							break
						}
						if validateControlIdentity(status, version, profile) != nil {
							continue
						}
						publishLatestControlStatus(updates, controlStatusDTO(status))
					}
				}
				_ = connection.Close()
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(300 * time.Millisecond):
			}
		}
	}()
	return updates
}

func publishLatestControlStatus(updates chan dto.LumenControlStatus, status dto.LumenControlStatus) {
	select {
	case updates <- status:
		return
	default:
	}
	select {
	case <-updates:
	default:
	}
	select {
	case updates <- status:
	default:
	}
}

func controlStatusDTO(status *controlv1.StatusSnapshot) dto.LumenControlStatus {
	result := dto.LumenControlStatus{
		Connected: true, Phase: controlPhaseDTO(status.GetPhase()),
		InferenceReady: status.GetPhase() == controlv1.Phase_PHASE_READY,
		Version:        status.GetVersion(), Backend: status.GetProfile(),
		StartedAtUnixMS: status.GetStartedAtUnixMs(), Sequence: status.GetSeq(),
	}
	if message := strings.TrimSpace(status.GetError()); message != "" {
		result.Error = &dto.Error{Code: dto.ErrorRuntimeNotReady, Message: message}
	}
	if download := status.GetDownload(); download != nil {
		result.Download = &dto.LumenDownloadProgress{
			Model: download.GetModel(), File: download.GetFile(),
			BytesDone: download.GetBytesDone(), BytesTotal: download.GetBytesTotal(),
			FilesDone: download.GetFilesDone(), FilesTotal: download.GetFilesTotal(),
		}
	}
	result.Services = make([]dto.LumenServiceStatus, 0, len(status.GetServices()))
	for _, service := range status.GetServices() {
		item := dto.LumenServiceStatus{Service: service.GetService(), Phase: controlPhaseDTO(service.GetPhase())}
		if message := strings.TrimSpace(service.GetError()); message != "" {
			item.Error = &dto.Error{Code: dto.ErrorRuntimeNotReady, Message: message}
		}
		result.Services = append(result.Services, item)
	}
	return result
}

func controlPhaseDTO(phase controlv1.Phase) dto.LumenControlPhase {
	switch phase {
	case controlv1.Phase_PHASE_STARTING:
		return dto.LumenControlStarting
	case controlv1.Phase_PHASE_DOWNLOADING:
		return dto.LumenControlDownloading
	case controlv1.Phase_PHASE_LOADING:
		return dto.LumenControlLoading
	case controlv1.Phase_PHASE_WARMUP:
		return dto.LumenControlWarmup
	case controlv1.Phase_PHASE_READY:
		return dto.LumenControlReady
	case controlv1.Phase_PHASE_FAILED:
		return dto.LumenControlFailed
	case controlv1.Phase_PHASE_STOPPING:
		return dto.LumenControlStopping
	default:
		return dto.LumenControlUnspecified
	}
}

func readControlLogs(ctx context.Context, endpoint string, backlog uint32, minLevel string) ([]dto.LumenLogEntry, error) {
	connection, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer connection.Close()
	stream, err := controlv1.NewControlClient(connection).TailLogs(ctx, &controlv1.TailLogsRequest{
		BacklogLines: backlog, MinLevel: minLevel, Follow: false,
	})
	if err != nil {
		return nil, err
	}
	logs := make([]dto.LumenLogEntry, 0, backlog)
	for {
		entry, receiveErr := stream.Recv()
		if errors.Is(receiveErr, io.EOF) {
			return logs, nil
		}
		if receiveErr != nil {
			return nil, receiveErr
		}
		logs = append(logs, dto.LumenLogEntry{
			TimeUnixMS: entry.GetTimeUnixMs(), Level: entry.GetLevel(), Target: entry.GetTarget(),
			Message: entry.GetMessage(), Fields: entry.GetFields(),
		})
	}
}

func monitorControlFailure(ctx context.Context, endpoint, version, profile string, stop context.CancelFunc) {
	probe := controlReadinessProbe(endpoint, version)
	if err := probe(ctx, profile, ""); err != nil {
		if ctx.Err() == nil && stop != nil {
			stop()
		}
		return
	}
	connection, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return
	}
	defer connection.Close()
	client := controlv1.NewControlClient(connection)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			probeCtx, cancel := context.WithTimeout(ctx, time.Second)
			status, callErr := client.GetStatus(probeCtx, &emptypb.Empty{})
			cancel()
			if callErr != nil {
				continue
			}
			if err := validateControlStatus(status, version, profile); err != nil {
				if stop != nil {
					stop()
				}
				return
			}
		}
	}
}

func releaseProfileBackend(profile string) string {
	switch {
	case strings.HasSuffix(profile, "-metal"):
		return "metal"
	case strings.HasSuffix(profile, "-gpu"):
		return "wgpu"
	case strings.HasSuffix(profile, "-cpu"):
		return "cpu"
	default:
		return ""
	}
}

var _ Factory = CurrentFactory{}
