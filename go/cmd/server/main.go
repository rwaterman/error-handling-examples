// Command server runs the orders API with an in-memory store.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rwaterman/error-handling-examples/go/app"
	"github.com/rwaterman/error-handling-examples/go/orders"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{ReplaceAttr: timeInUTC}))
	if err := run(*addr, logger); err != nil {
		logger.Error("server stopped", slog.Any("error", err))
		os.Exit(1)
	}
}

func timeInUTC(groups []string, attr slog.Attr) slog.Attr {
	if len(groups) == 0 && attr.Key == slog.TimeKey {
		return slog.Time(slog.TimeKey, attr.Value.Time().UTC())
	}
	return attr
}

func run(addr string, logger *slog.Logger) error {
	service := orders.NewService(orders.NewRepository(orders.NewMemoryStore()))
	server := &http.Server{
		Addr:              addr,
		Handler:           app.New(service, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	listenErr := make(chan error, 1)
	go func() {
		logger.Info("listening", slog.String("addr", addr))
		listenErr <- server.ListenAndServe()
	}()

	select {
	case err := <-listenErr:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-listenErr; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
