package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lukostrobl/fathom/internal/base"
	"github.com/lukostrobl/fathom/internal/config"
	"github.com/lukostrobl/fathom/internal/db"
	applog "github.com/lukostrobl/fathom/internal/log"
)

type Config struct {
	config.BasicConfig
	Database struct {
		URL string `koanf:"url"`
	} `koanf:"database"`
	Base BaseConfig `koanf:"base"`
}

// BaseConfig holds chain + endpoint configuration. Defaults are applied at use
// site (rpc.go / backfill.go) when a field is its zero value.
type BaseConfig struct {
	RPCURL               string `koanf:"rpc_url"`
	HyperSyncURL         string `koanf:"hypersync_url"`
	HyperSyncBearerToken string `koanf:"hypersync_bearer_token"`

	// Live tuning knobs (zero = default)
	ConfirmationDepth uint64 `koanf:"confirmation_depth"`
	PollIntervalMs    int    `koanf:"poll_interval_ms"`
	BlockBatchSize    uint64 `koanf:"block_batch_size"`
	Concurrency       int64  `koanf:"concurrency"`

	// Backfill tuning knob
	BatchCommitSize int `koanf:"batch_commit_size"`
}

func (c Config) GetBasicConfig() config.BasicConfig { return c.BasicConfig }

func main() {
	if err := run(); err != nil {
		// run() has already logged the error via slog; print to stderr and exit
		// non-zero so docker compose / shell-loop callers see the failure.
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return usageError()
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	// Help and command validation are answerable without parsing config or
	// opening a database connection — handle them up front so `--help` and
	// typos don't require a live DB.
	switch cmd {
	case "-h", "--help", "help":
		printUsage()
		return nil
	case "backfill", "live", "probe", "status":
		// known subcommand — fall through to setup below
	default:
		return fmt.Errorf("unknown subcommand %q\n\n%s", cmd, usageText())
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "local"
	}

	cfg, err := config.ParseConfig[Config]("base-collector", env)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	logger := applog.New(cfg.BasicConfig)

	// Bind ctrl-c / SIGTERM → ctx cancel for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer pool.Close()
	store := base.NewStore(pool)

	switch cmd {
	case "backfill":
		return runBackfillCmd(ctx, args, cfg, store, logger)
	case "live":
		return runLiveCmd(ctx, args, cfg, store, logger)
	case "probe":
		return runProbeCmd(ctx, args, cfg, logger)
	case "status":
		return runStatusCmd(ctx, store, logger)
	default:
		// unreachable: cmd validated above
		return fmt.Errorf("unknown subcommand %q", cmd)
	}
}

func runBackfillCmd(ctx context.Context, args []string, cfg Config, store *base.Store, logger *slog.Logger) error {
	fs := flag.NewFlagSet("backfill", flag.ExitOnError)
	fromBlock := fs.Uint64("from-block", 0, "first block to backfill (required, > 0)")
	toBlock := fs.Uint64("to-block", 0, "last block to backfill (0 = follow stream to its end)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *fromBlock == 0 {
		return errors.New("backfill: --from-block is required")
	}

	if cfg.Base.HyperSyncURL == "" {
		return errors.New("backfill: base.hypersync_url not configured")
	}
	fetcher := base.NewHTTPFetcher(cfg.Base.HyperSyncURL, cfg.Base.HyperSyncBearerToken)

	logger.Info(
		"base-collector: starting backfill",
		"from_block", *fromBlock,
		"to_block", *toBlock,
		"hypersync_url", cfg.Base.HyperSyncURL,
	)

	return base.RunBackfill(ctx, base.BackfillDeps{
		Fetcher:   fetcher,
		Store:     store,
		FromBlock: *fromBlock,
		ToBlock:   *toBlock,
	})
}

func runLiveCmd(ctx context.Context, args []string, cfg Config, store *base.Store, logger *slog.Logger) error {
	fs := flag.NewFlagSet("live", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.Base.RPCURL == "" {
		return errors.New("live: base.rpc_url not configured")
	}

	client, err := base.NewRPCClient(cfg.Base.RPCURL)
	if err != nil {
		return fmt.Errorf("open rpc client: %w", err)
	}
	defer client.Close()

	pollInterval := time.Duration(cfg.Base.PollIntervalMs) * time.Millisecond
	logger.Info(
		"base-collector: starting live tail",
		"rpc_url", cfg.Base.RPCURL,
		"poll_interval", pollInterval.String(),
		"confirmation_depth", cfg.Base.ConfirmationDepth,
	)

	return base.RunLive(ctx, base.LiveDeps{
		Client:            client,
		Store:             store,
		PollInterval:      pollInterval,
		BlockBatchSize:    cfg.Base.BlockBatchSize,
		Concurrency:       cfg.Base.Concurrency,
		ConfirmationDepth: cfg.Base.ConfirmationDepth,
	})
}

func runProbeCmd(ctx context.Context, args []string, cfg Config, logger *slog.Logger) error {
	fs := flag.NewFlagSet("probe", flag.ExitOnError)
	fromBlock := fs.Uint64("from-block", 0, "first block to probe (required, > 0)")
	toBlock := fs.Uint64("to-block", 0, "last block to probe (required, > from-block)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *fromBlock == 0 || *toBlock <= *fromBlock {
		return errors.New("probe: --from-block and --to-block are required; --to-block must exceed --from-block")
	}
	if cfg.Base.HyperSyncURL == "" {
		return errors.New("probe: base.hypersync_url not configured")
	}

	fetcher := base.NewHTTPFetcher(cfg.Base.HyperSyncURL, cfg.Base.HyperSyncBearerToken)
	report, err := base.RunProbe(ctx, base.ProbeDeps{
		Fetcher:   fetcher,
		FromBlock: *fromBlock,
		ToBlock:   *toBlock,
	})
	if err != nil {
		return err
	}
	report.Print(os.Stdout)
	logger.Info(
		"base-collector: probe complete",
		"from_block", *fromBlock,
		"to_block", *toBlock,
		"matched_events", report.MatchedEvents,
	)
	return nil
}

func runStatusCmd(ctx context.Context, store *base.Store, _ *slog.Logger) error {
	report, err := base.RunStatus(ctx, store)
	if err != nil {
		return err
	}
	report.Print(os.Stdout)
	return nil
}

func usageError() error {
	return errors.New(usageText())
}

func usageText() string {
	return `usage: base-collector <subcommand> [flags]

subcommands:
  backfill --from-block N [--to-block N]    one-shot HyperSync backfill
  live                                       long-running RPC tail
  probe    --from-block N --to-block N       dry-run HyperSync count, no writes
  status                                     print cursor + recent counts

config:  config/base-collector/{env}.toml + env vars (DATABASE__URL, BASE__*)
`
}

func printUsage() { _, _ = fmt.Fprint(os.Stdout, usageText()) }
