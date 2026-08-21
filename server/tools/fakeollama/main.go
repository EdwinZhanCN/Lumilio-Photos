// Command fakeollama is the deterministic, keyless Ollama boundary used only
// by the isolated browser E2E stack.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	listen := flag.String("listen", ":11434", "HTTP listen address")
	healthCheck := flag.String("health-check", "", "probe a fixture health URL and exit")
	flag.Parse()

	if *healthCheck != "" {
		client := &http.Client{Timeout: 3 * time.Second}
		response, err := client.Get(*healthCheck)
		if err != nil {
			log.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			log.Fatalf("fixture health returned %s", response.Status)
		}
		return
	}

	server := &http.Server{
		Addr:              *listen,
		Handler:           newFixtureServer(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	stopContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-stopContext.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()

	log.Printf("fakeollama listening on %s", *listen)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(fmt.Errorf("serve fakeollama: %w", err))
	}
}
