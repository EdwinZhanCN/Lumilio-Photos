package main

// Command sample records Docker container resource usage for an uploadbench
// run. It replaces the POSIX sampler while keeping the same CSV columns and
// positional arguments.

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const dockerFormat = "{{.Name}},{{.CPUPerc}},{{.MemUsage}},{{.MemPerc}},{{.NetIO}},{{.BlockIO}},{{.PIDs}}"

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <out_dir> [interval_seconds]\n", os.Args[0])
		os.Exit(2)
	}
	interval := 1.0
	if len(os.Args) == 3 {
		var err error
		interval, err = strconv.ParseFloat(os.Args[2], 64)
		if err != nil || interval <= 0 {
			fmt.Fprintln(os.Stderr, "interval_seconds must be a positive number")
			os.Exit(2)
		}
	}

	outDir := os.Args[1]
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create output directory: %s\n", err)
		os.Exit(1)
	}
	csvPath := filepath.Join(outDir, "resource_samples.csv")
	file, err := os.Create(csvPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create %s: %s\n", csvPath, err)
		os.Exit(1)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	if _, err := fmt.Fprintln(writer, "ts,container,cpu_pct,mem_used,mem_limit,mem_pct,net_io,block_io,pids"); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %s\n", csvPath, err)
		os.Exit(1)
	}
	if err := writer.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "flush %s: %s\n", csvPath, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "[sample] writing to %s (interval %gs); Ctrl-C to stop\n", csvPath, interval)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(time.Duration(interval * float64(time.Second)))
	defer ticker.Stop()
	for {
		if err := sample(ctx, writer); err != nil {
			fmt.Fprintf(os.Stderr, "sample: %s\n", err)
			os.Exit(1)
		}
		if err := writer.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "flush %s: %s\n", csvPath, err)
			os.Exit(1)
		}
		select {
		case <-ctx.Done():
			fmt.Fprintln(os.Stderr, "[sample] stopped")
			return
		case <-ticker.C:
		}
	}
}

func sample(ctx context.Context, writer *bufio.Writer) error {
	command := exec.CommandContext(ctx, "docker", "stats", "--no-stream", "--format", dockerFormat)
	command.Stderr = io.Discard
	output, err := command.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	timestamp := time.Now().UTC().Format(time.RFC3339)
	for _, rawLine := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if rawLine == "" {
			continue
		}
		fields := strings.SplitN(strings.TrimSuffix(rawLine, "\r"), ",", 7)
		if len(fields) != 7 {
			continue
		}
		memory := strings.SplitN(fields[2], " / ", 2)
		if len(memory) != 2 {
			continue
		}
		cpu := strings.TrimSuffix(fields[1], "%")
		memPct := strings.TrimSuffix(fields[3], "%")
		fmt.Fprintf(
			writer,
			"%s,%s,%s,%s,%s,%s,\"%s\",\"%s\",%s\n",
			timestamp,
			fields[0],
			cpu,
			memory[0],
			memory[1],
			memPct,
			csvEscape(fields[4]),
			csvEscape(fields[5]),
			fields[6],
		)
	}
	return nil
}

func csvEscape(value string) string {
	return strings.ReplaceAll(value, "\"", "\"\"")
}
