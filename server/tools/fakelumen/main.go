// Command fakelumen is the record/replay boundary for Lumen ML inference in
// tests and the isolated browser E2E stack. It implements the public Lumen
// gRPC contract in two modes:
//
//   - replay (default): serve recorded fixtures looked up by
//     (task, sha256(payload)). Without a recorded capability set it advertises
//     the deterministic builtin SigLIP capability and answers every semantic
//     request with one constant 768-dimensional vector — the legacy behavior.
//     Misses fall back to deterministic builtin responses and are counted per
//     task in the metrics endpoint; -strict turns a miss into an error.
//
//   - record (-record -upstream <addr> -fixtures <dir>): proxy every inference
//     to a real Lumen Hub, stream the answer back, and persist each exchange
//     as a reviewed fixture. The upstream capability set is persisted so a
//     later replay advertises exactly what the recording hub advertised.
//
// Product code exercises discovery, gRPC streaming, image preprocessing,
// queues, SQLite vector storage, retrieval, and best-frame selection either
// way; only model inference is replayed. See
// .agents/skills/lumilio-lumen-fixtures/SKILL.md for the workflow.
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	defaultAddress        = ":50051"
	defaultMetricsAddress = ":50052"
)

//go:embed all:fixtures
var embeddedFixtures embed.FS

func runHealthCheck(endpoint string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = pb.NewInferenceClient(conn).Health(ctx, &emptypb.Empty{})
	return err
}

type options struct {
	address        string
	metricsAddress string
	fixturesDir    string
	record         bool
	upstream       string
	strict         bool
}

func buildServer(opts options) (*inferenceServer, func(), error) {
	var store *fixtureStore
	var err error
	if opts.fixturesDir != "" {
		store, err = openFixtureDir(opts.fixturesDir)
	} else {
		var embedded fs.FS
		embedded, err = fs.Sub(embeddedFixtures, "fixtures")
		if err == nil {
			store, err = loadFixtureStore(embedded)
		}
	}
	if err != nil {
		return nil, nil, err
	}

	if !opts.record {
		return newReplayServer(store, opts.strict), func() {}, nil
	}

	if opts.upstream == "" {
		return nil, nil, errors.New("-record requires -upstream <host:port>")
	}
	if opts.fixturesDir == "" {
		return nil, nil, errors.New("-record requires -fixtures <dir> to persist recordings")
	}
	conn, err := grpc.NewClient(opts.upstream, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("dial upstream %s: %w", opts.upstream, err)
	}
	cleanup := func() { _ = conn.Close() }
	return newRecordServer(store, pb.NewInferenceClient(conn), opts.upstream), cleanup, nil
}

func run(opts options) error {
	listener, err := net.Listen("tcp", opts.address)
	if err != nil {
		return err
	}
	defer listener.Close()
	metricsListener, err := net.Listen("tcp", opts.metricsAddress)
	if err != nil {
		return err
	}
	defer metricsListener.Close()

	service, cleanup, err := buildServer(opts)
	if err != nil {
		return err
	}
	defer cleanup()

	server := grpc.NewServer()
	pb.RegisterInferenceServer(server, service)
	metricsServer := &http.Server{
		Handler:           metricsHandler(service),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveErrors := make(chan error, 2)
	go func() {
		serveErrors <- server.Serve(listener)
	}()
	go func() {
		serveErrors <- metricsServer.Serve(metricsListener)
	}()

	log.Printf(
		"fakelumen %s mode listening on %s (metrics %s, fixtures loaded %d)",
		service.mode,
		listener.Addr(),
		metricsListener.Addr(),
		service.store.size(),
	)
	if service.mode == modeRecord {
		log.Printf("recording upstream %s into %s", opts.upstream, opts.fixturesDir)
	}
	select {
	case <-ctx.Done():
	case err := <-serveErrors:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}

	server.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := metricsServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func main() {
	opts := options{}
	flag.StringVar(&opts.address, "listen", defaultAddress, "gRPC listen address")
	flag.StringVar(&opts.metricsAddress, "metrics-listen", defaultMetricsAddress, "HTTP metrics listen address")
	flag.StringVar(&opts.fixturesDir, "fixtures", "", "fixture directory (defaults to the embedded fixture set; required with -record)")
	flag.BoolVar(&opts.record, "record", false, "proxy inference to -upstream and persist fixtures")
	flag.StringVar(&opts.upstream, "upstream", "", "real Lumen Hub address for -record")
	flag.BoolVar(&opts.strict, "strict", false, "fail replay misses instead of serving the builtin fallback")
	healthCheck := flag.String("health-check", "", "probe an existing fixture endpoint")
	flag.Parse()

	var err error
	if *healthCheck != "" {
		err = runHealthCheck(*healthCheck)
	} else {
		err = run(opts)
	}
	if err != nil {
		log.Fatal(err)
	}
}
