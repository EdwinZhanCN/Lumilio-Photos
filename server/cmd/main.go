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
	"server/internal/agent/ref"
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
	if len(os.Args) > 1 && os.Args[1] == "config" {
		if err := runConfigCLI(os.Args[2:], os.Stdout, os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}
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
		PprofAddr:                    options.pprofAddr,
		AgentAuditLogPath:            options.agentAuditLogPath,
		AgentRefUserHotBudgetBytes:   options.agentRefUserHotBudgetMiB << 20,
		AgentRefGlobalHotBudgetBytes: options.agentRefGlobalHotBudgetMiB << 20,
		BreakGlass:                   envEnabled("LUMILIO_BREAK_GLASS"),
		BreakGlassUsername:           strings.TrimSpace(os.Getenv("LUMILIO_BREAK_GLASS_USERNAME")),
	}
	if err := app.Run(ctx, appConfig, controls); err != nil {
		fmt.Fprintf(os.Stderr, "server exited with error: %v\n", err)
		os.Exit(1)
	}
}

type cliOptions struct {
	configPath, pprofAddr, agentAuditLogPath             string
	agentRefUserHotBudgetMiB, agentRefGlobalHotBudgetMiB int64
}

func parseCLI(args []string, stderr io.Writer) (cliOptions, error) {
	const maxBudgetMiB int64 = (1<<63 - 1) >> 20

	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "path to the complete runtime TOML manifest (required)")
	pprofAddr := flags.String("pprof-addr", "", "listen address for this run's pprof server")
	agentAuditLog := flags.String("agent-audit-log", "", "append this run's LLM audit events to a JSONL file")
	agentRefUserHotBudgetMiB := flags.Int64("agent-ref-user-hot-budget-mib", ref.DefaultUserHotBudget>>20, "per-user Agent ref hot-memory budget in MiB")
	agentRefGlobalHotBudgetMiB := flags.Int64("agent-ref-global-hot-budget-mib", ref.DefaultGlobalHotBudget>>20, "global Agent ref hot-memory budget in MiB")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: server --config <path> [--pprof-addr <addr>] [--agent-audit-log <path>] [--agent-ref-user-hot-budget-mib <mib>] [--agent-ref-global-hot-budget-mib <mib>]")
	}
	if err := flags.Parse(args); err != nil {
		return cliOptions{}, err
	}
	if strings.TrimSpace(*configPath) == "" {
		flags.Usage()
		return cliOptions{}, fmt.Errorf("missing required --config <path>")
	}
	if *agentRefUserHotBudgetMiB <= 0 || *agentRefGlobalHotBudgetMiB <= 0 {
		return cliOptions{}, fmt.Errorf("Agent ref hot-memory budgets must be positive")
	}
	if *agentRefUserHotBudgetMiB > maxBudgetMiB || *agentRefGlobalHotBudgetMiB > maxBudgetMiB {
		return cliOptions{}, fmt.Errorf("Agent ref hot-memory budgets are too large")
	}
	if *agentRefGlobalHotBudgetMiB < *agentRefUserHotBudgetMiB {
		return cliOptions{}, fmt.Errorf("global Agent ref hot-memory budget must be greater than or equal to the per-user budget")
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return cliOptions{}, fmt.Errorf("unexpected positional arguments")
	}
	return cliOptions{
		configPath: strings.TrimSpace(*configPath), pprofAddr: strings.TrimSpace(*pprofAddr),
		agentAuditLogPath:          strings.TrimSpace(*agentAuditLog),
		agentRefUserHotBudgetMiB:   *agentRefUserHotBudgetMiB,
		agentRefGlobalHotBudgetMiB: *agentRefGlobalHotBudgetMiB,
	}, nil
}

func envEnabled(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}
