package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"server/app"
	"server/config"
)

// @title Lumilio-Photos API
// @version 1.0
// @description Media management system API with asset features
// @contact.name API Support
// @contact.url http://www.github.com/EdwinZhanCN/Lumilio-Photos
// @license.name GPLv3.0
// @license.url https://opensource.org/licenses/GPL-3.0
// @host localhost:6680
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
// @openapi 3.0.0
func main() {
	options, err := parseCLI(os.Args[1:], os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	appConfig, err := config.LoadAppConfig(options.configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load server configuration: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	controls := app.OperatorControls{
		PprofAddr:          options.pprofAddr,
		AgentAuditLogPath:  options.agentAuditLogPath,
		BreakGlass:         envEnabled("LUMILIO_BREAK_GLASS"),
		BreakGlassUsername: strings.TrimSpace(os.Getenv("LUMILIO_BREAK_GLASS_USERNAME")),
	}
	if err := app.Run(ctx, appConfig, controls); err != nil {
		fmt.Fprintf(os.Stderr, "server exited with error: %v\n", err)
		os.Exit(1)
	}
}

type cliOptions struct {
	configPath, pprofAddr, agentAuditLogPath string
}

func parseCLI(args []string, stderr io.Writer) (cliOptions, error) {
	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "path to the complete runtime TOML manifest (required)")
	pprofAddr := flags.String("pprof-addr", "", "listen address for this run's pprof server")
	agentAuditLog := flags.String("agent-audit-log", "", "append this run's LLM audit events to a JSONL file")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: server --config <path> [--pprof-addr <addr>] [--agent-audit-log <path>]")
	}
	if err := flags.Parse(args); err != nil {
		return cliOptions{}, err
	}
	if strings.TrimSpace(*configPath) == "" {
		flags.Usage()
		return cliOptions{}, fmt.Errorf("missing required --config <path>")
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return cliOptions{}, fmt.Errorf("unexpected positional arguments")
	}
	return cliOptions{configPath: strings.TrimSpace(*configPath), pprofAddr: strings.TrimSpace(*pprofAddr), agentAuditLogPath: strings.TrimSpace(*agentAuditLog)}, nil
}

func envEnabled(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}
