package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oti-adjei/zook/internal/config"
	"github.com/oti-adjei/zook/internal/server"
	"github.com/oti-adjei/zook/internal/wire"
)

// cmdServe runs the HTTP API + webhook daemon until SIGINT/SIGTERM.
func cmdServe(stdout, stderr io.Writer) int {
	addr := os.Getenv("ZOOK_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8484"
	}

	opts := server.Options{
		StacksRoot:    wire.StacksRoot(),
		APIToken:      os.Getenv("ZOOK_API_TOKEN"),
		WebhookSecret: []byte(os.Getenv("ZOOK_WEBHOOK_SECRET")),
		NewEngine: func(s config.Stack) server.Deployer {
			return wire.EngineFor(s, wire.DefaultTimeout())
		},
	}
	srv := server.New(opts)

	if opts.APIToken == "" {
		fmt.Fprintln(stderr, "warning: ZOOK_API_TOKEN not set — deploy/rollback endpoints are disabled")
	}
	if len(opts.WebhookSecret) == 0 {
		fmt.Fprintln(stderr, "warning: ZOOK_WEBHOOK_SECRET not set — webhook deliveries will be rejected")
	}

	httpSrv := &http.Server{Addr: addr, Handler: srv.Handler()}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.ListenAndServe() }()
	fmt.Fprintf(stdout, "zook serve listening on %s (stacks root %s)\n", addr, opts.StacksRoot)

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return 0
		}
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return 1
	case <-ctx.Done():
	}

	fmt.Fprintln(stdout, "shutting down…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(stderr, "graceful shutdown failed: %v\n", err)
		return 1
	}
	<-errCh
	fmt.Fprintln(stdout, "bye")
	return 0
}
