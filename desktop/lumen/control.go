package lumen

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	controlv1 "desktop/lumen/controlv1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Status is a desktop-friendly snapshot of the hub's control plane
// (lumen.control.v1). The hub binds its gRPC port before models are
// downloaded, so every startup phase is observable here.
type Status struct {
	Phase    string // "starting","downloading","loading","warmup","ready","failed","stopping"
	Version  string
	Profile  string
	Error    string
	Download *DownloadStatus
	Seq      uint64
}

// DownloadStatus reports the artifact currently transferring.
type DownloadStatus struct {
	Model      string `json:"model"`
	File       string `json:"file"`
	BytesDone  uint64 `json:"bytesDone"`
	BytesTotal uint64 `json:"bytesTotal"`
	FilesDone  uint32 `json:"filesDone"`
	FilesTotal uint32 `json:"filesTotal"`
}

// ErrControlUnsupported reports a hub build that predates the control plane;
// callers fall back to the TCP readiness probe.
var ErrControlUnsupported = errors.New("hub build does not expose the control plane")

// WatchStatus opens one status watch against the hub control plane and calls
// onUpdate for the current snapshot and every subsequent change. It returns
// nil when the stream ends cleanly, ctx.Err() on cancellation, and
// ErrControlUnsupported when the hub answers Unimplemented. Connection errors
// (hub process still binding) surface as regular errors — callers retry.
func WatchStatus(ctx context.Context, endpoint string, onUpdate func(Status)) error {
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	stream, err := controlv1.NewControlClient(conn).WatchStatus(ctx, &emptypb.Empty{})
	if err != nil {
		return watchErr(err)
	}
	for {
		snapshot, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return watchErr(err)
		}
		onUpdate(statusFromProto(snapshot))
	}
}

// LogTail fetches a one-shot structured log tail from the hub control plane
// (TailLogs with follow=false) rendered as plain text lines. Returns
// ErrControlUnsupported for hub builds that predate the control plane.
func LogTail(ctx context.Context, endpoint string, lines int) (string, error) {
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", err
	}
	defer conn.Close()

	stream, err := controlv1.NewControlClient(conn).TailLogs(ctx, &controlv1.TailLogsRequest{
		BacklogLines: uint32(lines),
		MinLevel:     "TRACE",
		Follow:       false,
	})
	if err != nil {
		return "", watchErr(err)
	}

	var out strings.Builder
	for {
		entry, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return out.String(), nil
			}
			return "", watchErr(err)
		}
		if out.Len() > 0 {
			out.WriteByte('\n')
		}
		timestamp := time.UnixMilli(entry.GetTimeUnixMs()).Format("2006-01-02 15:04:05")
		out.WriteString(timestamp)
		out.WriteByte(' ')
		out.WriteString(entry.GetLevel())
		out.WriteString(" [")
		out.WriteString(entry.GetTarget())
		out.WriteString("] ")
		out.WriteString(entry.GetMessage())
		for key, value := range entry.GetFields() {
			out.WriteByte(' ')
			out.WriteString(key)
			out.WriteByte('=')
			out.WriteString(value)
		}
	}
}

func watchErr(err error) error {
	if status.Code(err) == codes.Unimplemented {
		return ErrControlUnsupported
	}
	return err
}

func statusFromProto(snapshot *controlv1.StatusSnapshot) Status {
	out := Status{
		Phase:   phaseString(snapshot.GetPhase()),
		Version: snapshot.GetVersion(),
		Profile: snapshot.GetProfile(),
		Error:   snapshot.GetError(),
		Seq:     snapshot.GetSeq(),
	}
	if download := snapshot.GetDownload(); download != nil {
		out.Download = &DownloadStatus{
			Model:      download.GetModel(),
			File:       download.GetFile(),
			BytesDone:  download.GetBytesDone(),
			BytesTotal: download.GetBytesTotal(),
			FilesDone:  download.GetFilesDone(),
			FilesTotal: download.GetFilesTotal(),
		}
	}
	return out
}

func phaseString(phase controlv1.Phase) string {
	switch phase {
	case controlv1.Phase_PHASE_STARTING:
		return "starting"
	case controlv1.Phase_PHASE_DOWNLOADING:
		return "downloading"
	case controlv1.Phase_PHASE_LOADING:
		return "loading"
	case controlv1.Phase_PHASE_WARMUP:
		return "warmup"
	case controlv1.Phase_PHASE_READY:
		return "ready"
	case controlv1.Phase_PHASE_FAILED:
		return "failed"
	case controlv1.Phase_PHASE_STOPPING:
		return "stopping"
	default:
		return "starting"
	}
}
