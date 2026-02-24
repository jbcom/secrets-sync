package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/jbcom/extended-data-library/packages/secretssync/pkg/observability"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile     string
	logLevel    string
	logFormat   string
	metricsAddr string
	metricsPort int
)

// Build information set via ldflags at build time
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "secretsync",
	Short: "SecretSync - Multi-account secrets management",
	Long: `SecretSync synchronizes secrets from Vault to AWS across multiple accounts.

It supports:
- AWS Control Tower / Organizations for multi-account management
- Inheritance hierarchies (dev → staging → prod)
- Dynamic target discovery via Identity Center / Organizations
- Merge stores for centralized secret aggregation

Examples:
  # Run full pipeline
  secretsync pipeline --config config.yaml

  # Dry run for specific targets
  secretsync pipeline --config config.yaml --targets Serverless_Stg --dry-run

  # Merge only (no AWS sync)
  secretsync pipeline --config config.yaml --merge-only

  # Validate configuration
  secretsync validate --config config.yaml

  # Show dependency graph
  secretsync graph --config config.yaml`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Set log level
		level, err := log.ParseLevel(logLevel)
		if err != nil {
			level = log.InfoLevel
		}
		log.SetLevel(level)

		// Set log format
		if logFormat == "json" {
			log.SetFormatter(&log.JSONFormatter{})
		}

		// Start metrics server if enabled
		if metricsPort > 0 {
			go startMetricsServer()
		}
	},
}

// Execute runs the root command
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.yaml", "config file path")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "log format (text, json)")
	rootCmd.PersistentFlags().StringVar(&metricsAddr, "metrics-addr", "0.0.0.0", "metrics server address")
	rootCmd.PersistentFlags().IntVar(&metricsPort, "metrics-port", 0, "metrics server port (0 = disabled)")

	// Bind to viper
	viper.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log-level"))
	viper.BindPFlag("log.format", rootCmd.PersistentFlags().Lookup("log-format"))
	viper.BindPFlag("metrics.addr", rootCmd.PersistentFlags().Lookup("metrics-addr"))
	viper.BindPFlag("metrics.port", rootCmd.PersistentFlags().Lookup("metrics-port"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Default config locations
		viper.AddConfigPath(".")
		viper.AddConfigPath("/config")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// Environment variables
	viper.SetEnvPrefix("SECRETSYNC")
	viper.AutomaticEnv()
}

// startMetricsServer starts the Prometheus metrics HTTP server
func startMetricsServer() {
	addr := fmt.Sprintf("%s:%d", metricsAddr, metricsPort)

	mux := http.NewServeMux()
	mux.Handle("/metrics", observability.Handler())

	// Add health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.WithField("address", addr).Info("Starting metrics server")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.WithError(err).Error("Metrics server error")
	}
}
