package supervisor

import (
	"context"
	"errors"
	"net"
	"strings"

	serverconfig "server/config"
)

// RuntimePhase is the complete, externally observable Lumilio Server lifecycle.
// The Desktop host and optional Lumen Hub have separate lifetimes.
type RuntimePhase string

const (
	RuntimeStopped    RuntimePhase = "stopped"
	RuntimeStarting   RuntimePhase = "starting"
	RuntimeRunning    RuntimePhase = "running"
	RuntimeRestarting RuntimePhase = "restarting"
	RuntimeFailed     RuntimePhase = "failed"
)

// NetworkSummary is derived from a manifest accepted by the real Server config
// loader. The panel must not infer deployment security from URLs.
type NetworkSummary struct {
	Mode                   NetworkMode `json:"mode"`
	Listen                 string      `json:"listen"`
	PrimaryOrigin          string      `json:"primaryOrigin"`
	TLSMode                string      `json:"tlsMode"`
	ProxyMode              string      `json:"proxyMode"`
	PasskeyOrigin          string      `json:"passkeyOrigin"`
	RPID                   string      `json:"rpID"`
	PasskeyEnabled         bool        `json:"passkeyEnabled"`
	RemotePasskeyAvailable bool        `json:"remotePasskeyAvailable"`
}

// RuntimeSnapshot is the single source read by the panel, tray, and Server
// actions. ErrorMessage contains an actionable, bounded host error and never a
// secret value.
type RuntimeSnapshot struct {
	Phase                  RuntimePhase   `json:"phase"`
	Stage                  string         `json:"stage,omitempty"`
	ErrorCode              string         `json:"errorCode,omitempty"`
	ErrorMessage           string         `json:"errorMessage,omitempty"`
	BrowserURL             string         `json:"browserURL,omitempty"`
	CanOpen                bool           `json:"canOpen"`
	CanRestart             bool           `json:"canRestart"`
	LastKnownGoodAvailable bool           `json:"lastKnownGoodAvailable"`
	Network                NetworkSummary `json:"network"`
	OperationActive        bool           `json:"operationActive"`
}

type runtimeGeneration struct {
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

func initialRuntimeSnapshot() RuntimeSnapshot {
	return snapshotForPhase(RuntimeStopped)
}

func snapshotForPhase(phase RuntimePhase) RuntimeSnapshot {
	return RuntimeSnapshot{
		Phase:      phase,
		CanOpen:    phase == RuntimeRunning,
		CanRestart: phase == RuntimeRunning || phase == RuntimeFailed || phase == RuntimeStopped,
	}
}

func (s *Supervisor) RuntimeSnapshot() RuntimeSnapshot {
	s.snapshotMu.RLock()
	snapshot := s.snapshot
	s.snapshotMu.RUnlock()
	return snapshot
}

func (s *Supervisor) setSnapshot(snapshot RuntimeSnapshot) {
	snapshot.CanOpen = snapshot.Phase == RuntimeRunning && snapshot.BrowserURL != ""
	snapshot.CanRestart = snapshot.Phase == RuntimeRunning ||
		snapshot.Phase == RuntimeFailed ||
		snapshot.Phase == RuntimeStopped
	s.snapshotMu.Lock()
	s.snapshot = snapshot
	s.snapshotMu.Unlock()
	if s.onSnapshot != nil {
		s.onSnapshot(snapshot)
	}
}

func (s *Supervisor) updateSnapshot(update func(*RuntimeSnapshot)) {
	s.snapshotMu.Lock()
	snapshot := s.snapshot
	update(&snapshot)
	s.snapshot = snapshot
	s.snapshotMu.Unlock()
	if s.onSnapshot != nil {
		s.onSnapshot(snapshot)
	}
}

func networkSummaryFromConfig(cfg serverconfig.AppConfig) NetworkSummary {
	mode := NetworkLocal
	switch cfg.ServerConfig.TLS.Mode {
	case serverconfig.TLSModeExternal:
		mode = NetworkExternalHTTPS
	default:
		host, _, _ := net.SplitHostPort(cfg.ServerConfig.Listen)
		if host == "" || host == "0.0.0.0" || host == "::" {
			mode = NetworkLANHTTP
		}
	}
	return NetworkSummary{
		Mode:                   mode,
		Listen:                 cfg.ServerConfig.Listen,
		PrimaryOrigin:          cfg.ServerConfig.PrimaryOrigin,
		TLSMode:                string(cfg.ServerConfig.TLS.Mode),
		ProxyMode:              string(cfg.ServerConfig.Proxy.Mode),
		PasskeyOrigin:          cfg.Auth.PasskeyIdentity.Origin,
		RPID:                   cfg.Auth.PasskeyIdentity.RPID,
		PasskeyEnabled:         cfg.Auth.Passkey.Enabled,
		RemotePasskeyAvailable: cfg.Auth.Passkey.Enabled && cfg.ServerConfig.TLS.Mode == serverconfig.TLSModeExternal,
	}
}

func runtimeErrorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrAlreadyRunning):
		return "already_running"
	case errors.Is(err, ErrPortInUse):
		return "listen_unavailable"
	case errors.Is(err, ErrRuntimeStopTimeout):
		return "stop_timeout"
	case errors.Is(err, ErrRuntimeGenerationActive):
		return "generation_active"
	case errors.Is(err, ErrOperationInProgress):
		return "operation_in_progress"
	default:
		return "startup_failed"
	}
}

func runtimeErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	const maxRuntimeErrorRunes = 4096
	runes := []rune(message)
	if len(runes) > maxRuntimeErrorRunes {
		message = string(runes[:maxRuntimeErrorRunes]) + "…"
	}
	return message
}
