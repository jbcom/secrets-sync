package pipeline

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectVault(t *testing.T) {
	// Save and restore env
	origVaultAddr := os.Getenv("VAULT_ADDR")
	origVaultToken := os.Getenv("VAULT_TOKEN")
	origVaultRoleID := os.Getenv("VAULT_ROLE_ID")
	origVaultSecretID := os.Getenv("VAULT_SECRET_ID")
	defer func() {
		os.Setenv("VAULT_ADDR", origVaultAddr)
		os.Setenv("VAULT_TOKEN", origVaultToken)
		os.Setenv("VAULT_ROLE_ID", origVaultRoleID)
		os.Setenv("VAULT_SECRET_ID", origVaultSecretID)
	}()

	t.Run("no vault env", func(t *testing.T) {
		os.Unsetenv("VAULT_ADDR")
		os.Unsetenv("VAULT_TOKEN")

		d := detectVault()
		assert.False(t, d.Available)
	})

	t.Run("vault with token", func(t *testing.T) {
		os.Setenv("VAULT_ADDR", "http://vault:8200")
		os.Setenv("VAULT_TOKEN", "test-token")
		os.Unsetenv("VAULT_ROLE_ID")

		d := detectVault()
		assert.True(t, d.Available)
		assert.Equal(t, "http://vault:8200", d.Address)
		assert.Equal(t, "token", d.AuthType)
	})

	t.Run("vault with approle", func(t *testing.T) {
		os.Setenv("VAULT_ADDR", "http://vault:8200")
		os.Unsetenv("VAULT_TOKEN")
		os.Setenv("VAULT_ROLE_ID", "my-role")
		os.Setenv("VAULT_SECRET_ID", "my-secret")

		d := detectVault()
		assert.True(t, d.Available)
		assert.Equal(t, "approle", d.AuthType)
	})
}

func TestDetectAWS(t *testing.T) {
	// Save and restore env
	envVars := []string{
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
		"AWS_PROFILE", "AWS_REGION", "AWS_DEFAULT_REGION",
		"AWS_ROLE_ARN", "AWS_WEB_IDENTITY_TOKEN_FILE",
	}
	saved := make(map[string]string)
	for _, k := range envVars {
		saved[k] = os.Getenv(k)
	}
	defer func() {
		for k, v := range saved {
			if v != "" {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()

	// Clear all
	for _, k := range envVars {
		os.Unsetenv(k)
	}

	t.Run("no aws env", func(t *testing.T) {
		d := detectAWS()
		assert.False(t, d.Available)
	})

	t.Run("aws with env creds", func(t *testing.T) {
		os.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
		os.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
		os.Setenv("AWS_REGION", "us-west-2")

		d := detectAWS()
		assert.True(t, d.Available)
		assert.Equal(t, "us-west-2", d.Region)
		assert.Equal(t, "env", d.AuthType)

		os.Unsetenv("AWS_ACCESS_KEY_ID")
		os.Unsetenv("AWS_SECRET_ACCESS_KEY")
		os.Unsetenv("AWS_REGION")
	})

	t.Run("aws with profile", func(t *testing.T) {
		os.Setenv("AWS_PROFILE", "production")
		os.Setenv("AWS_DEFAULT_REGION", "eu-west-1")

		d := detectAWS()
		assert.True(t, d.Available)
		assert.Equal(t, "eu-west-1", d.Region)
		assert.Equal(t, "profile", d.AuthType)

		os.Unsetenv("AWS_PROFILE")
		os.Unsetenv("AWS_DEFAULT_REGION")
	})

	t.Run("aws default region", func(t *testing.T) {
		os.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
		os.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
		// No region set - should default

		d := detectAWS()
		assert.True(t, d.Available)
		assert.Equal(t, "us-east-1", d.Region) // Default

		os.Unsetenv("AWS_ACCESS_KEY_ID")
		os.Unsetenv("AWS_SECRET_ACCESS_KEY")
	})
}

func TestApplyAutoDetection(t *testing.T) {
	t.Run("applies vault detection", func(t *testing.T) {
		cfg := &Config{}
		detected := DetectedClients{
			Vault: VaultDetection{
				Available: true,
				Address:   "http://auto-vault:8200",
				AuthType:  "token",
			},
		}

		// Set env for token
		origToken := os.Getenv("VAULT_TOKEN")
		os.Setenv("VAULT_TOKEN", "auto-token")
		defer os.Setenv("VAULT_TOKEN", origToken)

		cfg.ApplyAutoDetection(detected)

		assert.Equal(t, "http://auto-vault:8200", cfg.Vault.Address)
		assert.NotNil(t, cfg.Vault.Auth.Token)
		assert.Equal(t, "auto-token", cfg.Vault.Auth.Token.Token)
	})

	t.Run("applies aws detection", func(t *testing.T) {
		cfg := &Config{}
		detected := DetectedClients{
			AWS: AWSDetection{
				Available: true,
				Region:    "ap-southeast-1",
				AuthType:  "env",
			},
		}

		cfg.ApplyAutoDetection(detected)

		assert.Equal(t, "ap-southeast-1", cfg.AWS.Region)
	})

	t.Run("does not override explicit config", func(t *testing.T) {
		cfg := &Config{
			Vault: VaultConfig{
				Address: "http://explicit:8200",
			},
			AWS: AWSConfig{
				Region: "explicit-region",
			},
		}
		detected := DetectedClients{
			Vault: VaultDetection{
				Available: true,
				Address:   "http://auto-vault:8200",
			},
			AWS: AWSDetection{
				Available: true,
				Region:    "auto-region",
			},
		}

		cfg.ApplyAutoDetection(detected)

		// Should keep explicit values
		assert.Equal(t, "http://explicit:8200", cfg.Vault.Address)
		assert.Equal(t, "explicit-region", cfg.AWS.Region)
	})

	t.Run("auto-configures merge store", func(t *testing.T) {
		cfg := &Config{}
		detected := DetectedClients{
			Vault: VaultDetection{
				Available: true,
				Address:   "http://vault:8200",
			},
		}

		cfg.ApplyAutoDetection(detected)

		assert.NotNil(t, cfg.MergeStore.Vault)
		assert.Equal(t, "merged-secrets", cfg.MergeStore.Vault.Mount)
	})
}

func TestAutoDetectAndConfigure(t *testing.T) {
	// Save and restore env
	origVaultAddr := os.Getenv("VAULT_ADDR")
	origVaultToken := os.Getenv("VAULT_TOKEN")
	defer func() {
		if origVaultAddr != "" {
			os.Setenv("VAULT_ADDR", origVaultAddr)
		} else {
			os.Unsetenv("VAULT_ADDR")
		}
		if origVaultToken != "" {
			os.Setenv("VAULT_TOKEN", origVaultToken)
		} else {
			os.Unsetenv("VAULT_TOKEN")
		}
	}()

	t.Run("full auto-configuration", func(t *testing.T) {
		os.Setenv("VAULT_ADDR", "http://test-vault:8200")
		os.Setenv("VAULT_TOKEN", "test-token")

		cfg := &Config{
			Targets: map[string]Target{
				"Production": {Imports: []string{"analytics"}},
			},
		}

		detected := cfg.AutoDetectAndConfigure()

		assert.True(t, detected.Vault.Available)
		assert.Equal(t, "http://test-vault:8200", cfg.Vault.Address)
		assert.NotNil(t, cfg.MergeStore.Vault)
		// AutoConfigure should create placeholder source
		assert.Contains(t, cfg.Sources, "analytics")
	})
}

func TestIsKubernetes(t *testing.T) {
	// Save and restore
	origHost := os.Getenv("KUBERNETES_SERVICE_HOST")
	defer func() {
		if origHost != "" {
			os.Setenv("KUBERNETES_SERVICE_HOST", origHost)
		} else {
			os.Unsetenv("KUBERNETES_SERVICE_HOST")
		}
	}()

	t.Run("not kubernetes", func(t *testing.T) {
		os.Unsetenv("KUBERNETES_SERVICE_HOST")
		assert.False(t, isKubernetes())
	})

	t.Run("is kubernetes via env", func(t *testing.T) {
		os.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
		assert.True(t, isKubernetes())
	})
}

func TestGetEnvOrDefault(t *testing.T) {
	origVal := os.Getenv("TEST_ENV_VAR_FOR_UNIT_TEST")
	defer func() {
		if origVal != "" {
			os.Setenv("TEST_ENV_VAR_FOR_UNIT_TEST", origVal)
		} else {
			os.Unsetenv("TEST_ENV_VAR_FOR_UNIT_TEST")
		}
	}()

	t.Run("returns env value when set", func(t *testing.T) {
		os.Setenv("TEST_ENV_VAR_FOR_UNIT_TEST", "custom-value")
		result := getEnvOrDefault("TEST_ENV_VAR_FOR_UNIT_TEST", "default")
		assert.Equal(t, "custom-value", result)
	})

	t.Run("returns default when not set", func(t *testing.T) {
		os.Unsetenv("TEST_ENV_VAR_FOR_UNIT_TEST")
		result := getEnvOrDefault("TEST_ENV_VAR_FOR_UNIT_TEST", "default")
		assert.Equal(t, "default", result)
	})
}
