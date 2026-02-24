// Package pipeline provides unified configuration and orchestration for secrets syncing pipelines.
// It supports AWS Control Tower / Organizations patterns for multi-account secrets management.
package pipeline

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// LoadConfig loads configuration from file with auto-detection and resolution.
//
// Auto-detection: Clients are automatically enabled based on environment:
//   - Vault: VAULT_ADDR, VAULT_TOKEN, VAULT_ROLE_ID/SECRET_ID
//   - AWS: AWS_ACCESS_KEY_ID, AWS_PROFILE, IAM roles, etc.
//
// Minimal config example (everything else auto-detected):
//
//	targets:
//	  Production:
//	    imports: [analytics, data-engineers]
//
// The system will:
//  1. Auto-detect Vault/AWS from environment
//  2. Resolve sources/targets via fuzzy matching against AWS Organizations
//  3. Configure merge store automatically
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	cfg.applyDefaults()
	cfg.expandEnvVars()

	// Auto-detect clients from environment and apply
	cfg.AutoDetectAndConfigure()

	// Also load via Viper for explicit env var overrides
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("SECRETSYNC")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicit overrides take precedence
	if v.IsSet("log.level") {
		cfg.Log.Level = v.GetString("log.level")
	}
	if v.IsSet("aws.region") {
		cfg.AWS.Region = v.GetString("aws.region")
	}
	if v.IsSet("vault.address") {
		cfg.Vault.Address = v.GetString("vault.address")
	}

	return &cfg, nil
}

// LoadConfigWithoutAutoDetect loads config without auto-detection (for testing)
func LoadConfigWithoutAutoDetect(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	cfg.applyDefaults()
	cfg.expandEnvVars()

	return &cfg, nil
}

// applyDefaults sets default values for unset fields
func (c *Config) applyDefaults() {
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Log.Format == "" {
		c.Log.Format = "text"
	}
	if c.AWS.Region == "" {
		c.AWS.Region = "us-east-1"
	}
	if c.AWS.ControlTower.ExecutionRole.Name == "" {
		c.AWS.ControlTower.ExecutionRole.Name = "AWSControlTowerExecution"
	}
	if c.Pipeline.Merge.Parallel <= 0 {
		c.Pipeline.Merge.Parallel = 4
	}
	if c.Pipeline.Sync.Parallel <= 0 {
		c.Pipeline.Sync.Parallel = 4
	}
}

// expandEnvVars expands ${VAR} patterns in config values
func (c *Config) expandEnvVars() {
	envPattern := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	const maxEnvValueLength = 10000

	expand := func(s string) string {
		return envPattern.ReplaceAllStringFunc(s, func(match string) string {
			varName := match[2 : len(match)-1]
			if val := os.Getenv(varName); val != "" {
				if len(val) > maxEnvValueLength {
					log.WithField("variable", varName).Warn("Environment variable value exceeds maximum length, keeping placeholder")
					return match
				}
				return val
			}
			return match
		})
	}

	if c.Vault.Auth.AppRole != nil {
		c.Vault.Auth.AppRole.RoleID = expand(c.Vault.Auth.AppRole.RoleID)
		c.Vault.Auth.AppRole.SecretID = expand(c.Vault.Auth.AppRole.SecretID)
	}
	if c.Vault.Auth.Token != nil {
		c.Vault.Auth.Token.Token = expand(c.Vault.Auth.Token.Token)
	}
}

// Validate validates the configuration with minimal requirements.
// The system auto-detects and resolves most configuration via:
// - AWS Organizations discovery for account resolution
// - Fuzzy name matching for source/target identification
// - Auto-detection of Vault vs AWS based on what's available
func (c *Config) Validate() error {
	// Must have at least one target or dynamic_target
	if len(c.Targets) == 0 && len(c.DynamicTargets) == 0 {
		return fmt.Errorf("at least one target or dynamic_target is required")
	}

	// Validate S3 merge store if explicitly configured
	if c.MergeStore.S3 != nil && c.MergeStore.S3.Bucket == "" {
		return fmt.Errorf("merge_store.s3.bucket is required when using S3 merge store")
	}

	// Validate target account_id format IF explicitly provided
	// (account_id is NOT required - can be resolved via fuzzy matching)
	for name, target := range c.Targets {
		if target.AccountID != "" && !isValidAWSAccountID(target.AccountID) {
			return fmt.Errorf("target %q: invalid account_id format %q (must be 12 digits)", name, target.AccountID)
		}
		// Note: imports are NOT validated here - they can be resolved dynamically
		// via fuzzy matching against AWS Organizations or Vault mounts
	}

	// Validate inheritance if targets reference each other
	if err := c.ValidateTargetInheritance(); err != nil {
		return err
	}

	// Validate dynamic target patterns if present
	for name, dt := range c.DynamicTargets {
		// Discovery config is optional - can default to Organizations auto-discovery
		if dt.Discovery.Organizations != nil && dt.Discovery.Organizations.NameMatching != nil {
			nm := dt.Discovery.Organizations.NameMatching
			if nm.Strategy != "" && nm.Strategy != "exact" && nm.Strategy != "fuzzy" && nm.Strategy != "loose" {
				return fmt.Errorf("dynamic_target %q: invalid name_matching.strategy %q (must be exact, fuzzy, or loose)", name, nm.Strategy)
			}
		}
		// Validate account_name_patterns regex if present
		for i, pattern := range dt.AccountNamePatterns {
			if pattern.Pattern != "" {
				if _, err := regexp.Compile(pattern.Pattern); err != nil {
					return fmt.Errorf("dynamic_target %q: account_name_patterns[%d].pattern is invalid regex: %w", name, i, err)
				}
			}
		}
	}

	return nil
}

// AutoConfigure applies intelligent defaults and resolves unspecified configuration.
// Call this after loading config but before validation to fill in gaps.
func (c *Config) AutoConfigure() {
	// Auto-detect merge store if not specified
	// MergeStore is a value type, so check if both sub-configs are nil
	if c.MergeStore.Vault == nil && c.MergeStore.S3 == nil {
		if c.Vault.Address != "" {
			// Default to Vault merge store if Vault is configured
			c.MergeStore.Vault = &MergeStoreVault{Mount: "merged-secrets"}
			log.Info("Auto-configured Vault merge store (merged-secrets)")
		}
		// If no Vault and no S3, merge store will be configured during pipeline init
		// based on available auth
	}

	// Initialize sources map if nil
	if c.Sources == nil {
		c.Sources = make(map[string]Source)
	}

	// Auto-create source entries for any imports that don't exist
	// These will be resolved later via fuzzy matching
	for _, target := range c.Targets {
		for _, imp := range target.Imports {
			// Skip if it's another target (inheritance)
			if _, isTarget := c.Targets[imp]; isTarget {
				continue
			}
			// Create placeholder source if doesn't exist
			if _, exists := c.Sources[imp]; !exists {
				c.Sources[imp] = Source{} // Will be resolved via fuzzy matching
				log.WithField("source", imp).Debug("Auto-created placeholder source for resolution")
			}
		}
	}

	// Same for dynamic targets
	for _, dt := range c.DynamicTargets {
		for _, imp := range dt.Imports {
			if _, isTarget := c.Targets[imp]; isTarget {
				continue
			}
			if _, exists := c.Sources[imp]; !exists {
				c.Sources[imp] = Source{}
			}
		}
	}
}

// WriteConfig writes the configuration to a file
func (c *Config) WriteConfig(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// isValidAWSAccountID validates that an AWS account ID is exactly 12 digits
func isValidAWSAccountID(accountID string) bool {
	if len(accountID) != 12 {
		return false
	}
	for _, c := range accountID {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
