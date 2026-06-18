// Command anatomy serves the Anatomy dossier API and embedded frontend.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lukostrobl/fathom/internal/anatomy"
	"github.com/lukostrobl/fathom/internal/config"
	"github.com/lukostrobl/fathom/internal/db"
	applog "github.com/lukostrobl/fathom/internal/log"
	anatomyweb "github.com/lukostrobl/fathom/web/anatomy"
)

type Config struct {
	config.BasicConfig
	Database struct {
		URL string `koanf:"url"`
	} `koanf:"database"`
}

func (c Config) GetBasicConfig() config.BasicConfig { return c.BasicConfig }

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", ":8090", "listen address")
	flag.Parse()

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "local"
	}
	cfg, err := config.ParseConfig[Config]("anatomy", env)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	logger := applog.New(cfg.BasicConfig)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer pool.Close()

	srv := anatomy.NewServer(
		anatomy.NewPgDossier(pool),
		anatomy.NewPgStats(pool),
		anatomyweb.Assets(),
		logger,
	)
	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	}()

	logger.Info("anatomy: listening", "addr", *addr)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
