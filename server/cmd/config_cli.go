package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"server/config"
)

func runConfigCLI(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: server config <init|validate|healthcheck> [options]")
	}
	switch args[0] {
	case "init":
		return runConfigInit(args[1:], stdout, stderr)
	case "validate":
		return runConfigValidate(args[1:], stdout, stderr)
	case "healthcheck":
		return runConfigHealthcheck(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown config command %q (want init, validate, or healthcheck)", args[0])
	}
}

type repeatedStrings []string

func (values *repeatedStrings) String() string { return strings.Join(*values, ",") }
func (values *repeatedStrings) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func runConfigInit(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("server config init", flag.ContinueOnError)
	flags.SetOutput(stderr)
	profile := flags.String("profile", "", strings.Join(config.ProfileNames(true), " or "))
	origin := flags.String("origin", "", "canonical public https origin")
	email := flags.String("email", "", "ACME account email (docker-acme)")
	output := flags.String("output", "", "destination server.toml")
	force := flags.Bool("force", false, "replace an existing output file")
	stateDir := flags.String("state-dir", "/data/app-state", "container application state path")
	storageDir := flags.String("storage-dir", "/data/storage", "container media storage path")
	var trustedProxies repeatedStrings
	flags.Var(&trustedProxies, "trusted-proxy", "trusted proxy CIDR (repeatable)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("config init does not accept positional arguments")
	}
	if strings.TrimSpace(*profile) == "" || strings.TrimSpace(*origin) == "" || strings.TrimSpace(*output) == "" {
		return errors.New("config init requires --profile, --origin, and --output")
	}

	data, err := config.GenerateManifest(
		config.ProfileName(strings.TrimSpace(*profile)),
		config.ProfileInputs{
			Origin:            strings.TrimSpace(*origin),
			Email:             strings.TrimSpace(*email),
			TrustedProxyCIDRs: trustedProxies,
			StateDir:          strings.TrimSpace(*stateDir),
			StorageDir:        strings.TrimSpace(*storageDir),
		},
	)
	if err != nil {
		return err
	}
	path, err := filepath.Abs(strings.TrimSpace(*output))
	if err != nil {
		return err
	}
	if !*force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("refusing to overwrite existing config %s (use --force)", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect output %s: %w", path, err)
		}
	}
	if err := writeConfigAtomic(path, data); err != nil {
		return err
	}
	cfg, err := config.LoadAppConfig(path)
	if err != nil {
		return errors.Join(fmt.Errorf("generated config failed strict reload: %w", err), os.Remove(path))
	}
	fmt.Fprintf(stdout, "wrote %s\n", path)
	writeConfigReport(stdout, cfg)
	return nil
}

func writeConfigAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".server.toml-*")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("install config %s: %w", path, err)
	}
	return nil
}

func runConfigValidate(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("server config validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("config", "", "complete runtime TOML manifest")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*path) == "" {
		return errors.New("config validate requires --config <path>")
	}
	cfg, err := config.LoadAppConfig(strings.TrimSpace(*path))
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, "configuration valid")
	writeConfigReport(stdout, cfg)
	return nil
}

func writeConfigReport(output io.Writer, cfg config.AppConfig) {
	fmt.Fprintf(output, "primary origin: %s\n", cfg.ServerConfig.PrimaryOrigin)
	fmt.Fprintf(output, "passkey RP ID: %s\n", cfg.Auth.PasskeyIdentity.RPID)
	fmt.Fprintf(output, "application listener: %s\n", cfg.ServerConfig.Listen)
	fmt.Fprintf(output, "TLS owner: %s\n", cfg.ServerConfig.TLS.Mode)
	fmt.Fprintf(output, "proxy mode: %s\n", cfg.ServerConfig.Proxy.Mode)
	if cfg.ServerConfig.TLS.Mode == config.TLSModeACME {
		fmt.Fprintf(output, "ACME HTTP listener: %s\n", cfg.ServerConfig.TLS.HTTPListen)
		fmt.Fprintln(output, "required host ports: TCP 80 -> ACME HTTP listener; TCP 443 -> application listener")
	}
	if cfg.ServerConfig.TLS.Mode == config.TLSModeExternal {
		fmt.Fprintf(output, "trusted proxy CIDRs: %s\n", prefixesString(cfg.ServerConfig.Proxy.TrustedCIDRs))
		fmt.Fprintln(output, "required host ports: publish HTTPS on the reverse proxy; do not publish the application listener")
	}
	if cfg.ServerConfig.TLS.Mode == config.TLSModeOff {
		fmt.Fprintln(output, "security warning: plaintext HTTP; use only for local development or an explicitly accepted LAN profile")
	}
}

func prefixesString(prefixes []netip.Prefix) string {
	values := make([]string, len(prefixes))
	for index, prefix := range prefixes {
		values[index] = prefix.String()
	}
	return strings.Join(values, ", ")
}

func runConfigHealthcheck(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("server config healthcheck", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("config", "", "complete runtime TOML manifest")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*path) == "" {
		return errors.New("config healthcheck requires --config <path>")
	}
	cfg, err := config.LoadAppConfig(strings.TrimSpace(*path))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := checkRuntimeHealth(ctx, cfg.ServerConfig); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "ready")
	return nil
}

func checkRuntimeHealth(ctx context.Context, server config.ServerConfig) error {
	dialAddress, err := localDialAddress(server.Listen)
	if err != nil {
		return err
	}
	requestURL := "http://" + dialAddress + "/api/v1/health/ready"
	transport := &http.Transport{}
	if server.TLS.Mode == config.TLSModeACME {
		requestURL = server.PrimaryOrigin + "/api/v1/health/ready"
		primary, _ := url.Parse(server.PrimaryOrigin)
		transport.TLSClientConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: primary.Hostname(),
		}
		transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, dialAddress)
		}
	}
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("readiness request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("readiness returned HTTP %d", response.StatusCode)
	}
	return nil
}

func localDialAddress(listen string) (string, error) {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", err
	}
	ip := net.ParseIP(host)
	if host == "" || (ip != nil && ip.IsUnspecified()) {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port), nil
}
