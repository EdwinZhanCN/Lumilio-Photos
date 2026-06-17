package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"server/app"
	"server/config"

	"github.com/joho/godotenv"
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
	// Cancel the application context on SIGINT/SIGTERM so app.Run can perform a
	// graceful shutdown. The bootstrap itself lives in server/app so it can also
	// be driven in-process by the desktop supervisor.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	loadEnvFiles()
	appConfig, err := config.LoadAppConfigWithOptions(config.LoadOptions{
		Environment: os.Getenv("SERVER_ENV"),
		ConfigFile:  os.Getenv("SERVER_CONFIG_FILE"),
		Env:         config.ProcessEnv(),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "load server configuration: %v\n", err)
		os.Exit(1)
	}

	if err := app.Run(ctx, appConfig); err != nil {
		fmt.Fprintf(os.Stderr, "server exited with error: %v\n", err)
		os.Exit(1)
	}
}

func loadEnvFiles() {
	_ = godotenv.Load(".env")
	if os.Getenv("SERVER_ENV") == "development" {
		_ = godotenv.Load(".env.development")
	}
}
