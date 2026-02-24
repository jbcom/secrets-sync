// Package pipeline provides automatic detection of available clients and features.
// Clients are auto-enabled based on environment variables - no explicit config required.
package pipeline

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

// DetectedClients represents which clients are available based on environment
type DetectedClients struct {
	// Vault client availability
	Vault VaultDetection
	// AWS client availability
	AWS AWSDetection
}

// VaultDetection contains Vault auto-detection results
type VaultDetection struct {
	Available bool
	Address   string
	AuthType  string // token, approle, kubernetes, aws-iam
}

// AWSDetection contains AWS auto-detection results
type AWSDetection struct {
	Available     bool
	Region        string
	AuthType      string // env, iam-role, profile, sso
	HasOrgsAccess bool   // Can we access Organizations API?
}

// AutoDetectClients detects available clients from environment
func AutoDetectClients() DetectedClients {
	l := log.WithField("action", "AutoDetectClients")

	detected := DetectedClients{}

	// Detect Vault
	detected.Vault = detectVault()
	if detected.Vault.Available {
		l.WithFields(log.Fields{
			"address":  detected.Vault.Address,
			"authType": detected.Vault.AuthType,
		}).Info("Vault client auto-detected")
	}

	// Detect AWS
	detected.AWS = detectAWS()
	if detected.AWS.Available {
		l.WithFields(log.Fields{
			"region":   detected.AWS.Region,
			"authType": detected.AWS.AuthType,
		}).Info("AWS client auto-detected")
	}

	return detected
}

// detectVault checks for Vault availability via environment
func detectVault() VaultDetection {
	d := VaultDetection{}

	// Check VAULT_ADDR - primary indicator
	if addr := os.Getenv("VAULT_ADDR"); addr != "" {
		d.Available = true
		d.Address = addr

		// Detect auth type
		if os.Getenv("VAULT_TOKEN") != "" {
			d.AuthType = "token"
		} else if os.Getenv("VAULT_ROLE_ID") != "" && os.Getenv("VAULT_SECRET_ID") != "" {
			d.AuthType = "approle"
		} else if os.Getenv("VAULT_ROLE") != "" && isKubernetes() {
			d.AuthType = "kubernetes"
		} else if os.Getenv("VAULT_AWS_ROLE") != "" {
			d.AuthType = "aws-iam"
		} else {
			// Default to token - might be in ~/.vault-token
			d.AuthType = "token"
		}
	}

	return d
}

// detectAWS checks for AWS availability via environment or metadata
func detectAWS() AWSDetection {
	d := AWSDetection{}

	// Check standard AWS environment variables
	hasEnvCreds := os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != ""
	hasProfile := os.Getenv("AWS_PROFILE") != ""
	hasRoleARN := os.Getenv("AWS_ROLE_ARN") != ""
	hasWebIdentity := os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE") != ""
	hasSSO := os.Getenv("AWS_SSO_START_URL") != ""

	// Running on EC2/ECS/Lambda with IAM role
	isEC2 := isEC2Instance()
	isECS := os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI") != "" ||
		os.Getenv("AWS_CONTAINER_CREDENTIALS_FULL_URI") != ""
	isLambda := os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != ""

	if hasEnvCreds || hasProfile || hasRoleARN || hasWebIdentity || hasSSO || isEC2 || isECS || isLambda {
		d.Available = true

		// Detect region
		if region := os.Getenv("AWS_REGION"); region != "" {
			d.Region = region
		} else if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
			d.Region = region
		} else {
			d.Region = "us-east-1" // Default
		}

		// Detect auth type
		switch {
		case hasEnvCreds:
			d.AuthType = "env"
		case hasProfile:
			d.AuthType = "profile"
		case hasSSO:
			d.AuthType = "sso"
		case hasWebIdentity:
			d.AuthType = "web-identity"
		case hasRoleARN:
			d.AuthType = "assume-role"
		case isLambda || isECS:
			d.AuthType = "iam-role"
		case isEC2:
			d.AuthType = "ec2-metadata"
		}
	}

	return d
}

// isKubernetes checks if we're running in Kubernetes
func isKubernetes() bool {
	// Check for Kubernetes service account token
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
		return true
	}
	// Check for KUBERNETES_SERVICE_HOST
	return os.Getenv("KUBERNETES_SERVICE_HOST") != ""
}

// isEC2Instance checks if we're running on EC2 (simple heuristic)
func isEC2Instance() bool {
	// Check for EC2 metadata endpoint availability indicator
	// In real impl, would try to reach 169.254.169.254
	// For now, check common EC2 env indicators
	return os.Getenv("EC2_INSTANCE_ID") != "" ||
		os.Getenv("AWS_EXECUTION_ENV") != ""
}

// ApplyAutoDetection updates config based on auto-detected clients
func (c *Config) ApplyAutoDetection(detected DetectedClients) {
	l := log.WithField("action", "ApplyAutoDetection")

	// Apply Vault detection if not explicitly configured
	if c.Vault.Address == "" && detected.Vault.Available {
		c.Vault.Address = detected.Vault.Address
		l.WithField("address", detected.Vault.Address).Info("Applied auto-detected Vault address")

		// Set auth if not configured
		if c.Vault.Auth.Token == nil && c.Vault.Auth.AppRole == nil && c.Vault.Auth.Kubernetes == nil {
			switch detected.Vault.AuthType {
			case "token":
				token := os.Getenv("VAULT_TOKEN")
				if token == "" {
					// Try to read from file
					if data, err := os.ReadFile(os.ExpandEnv("$HOME/.vault-token")); err == nil {
						token = strings.TrimSpace(string(data))
					}
				}
				if token != "" {
					c.Vault.Auth.Token = &TokenAuth{Token: token}
				}
			case "approle":
				c.Vault.Auth.AppRole = &AppRoleAuth{
					RoleID:   os.Getenv("VAULT_ROLE_ID"),
					SecretID: os.Getenv("VAULT_SECRET_ID"),
					Mount:    getEnvOrDefault("VAULT_APPROLE_MOUNT", "approle"),
				}
			case "kubernetes":
				c.Vault.Auth.Kubernetes = &KubernetesAuth{
					Role:      os.Getenv("VAULT_ROLE"),
					MountPath: getEnvOrDefault("VAULT_K8S_MOUNT", "kubernetes"),
				}
			}
		}

		// Set namespace if present
		if c.Vault.Namespace == "" {
			if ns := os.Getenv("VAULT_NAMESPACE"); ns != "" {
				c.Vault.Namespace = ns
			}
		}
	}

	// Apply AWS detection if not explicitly configured
	if c.AWS.Region == "" && detected.AWS.Available {
		c.AWS.Region = detected.AWS.Region
		l.WithField("region", detected.AWS.Region).Info("Applied auto-detected AWS region")
	}

	// Auto-configure merge store if not set
	if c.MergeStore.Vault == nil && c.MergeStore.S3 == nil {
		if detected.Vault.Available {
			c.MergeStore.Vault = &MergeStoreVault{Mount: "merged-secrets"}
			l.Info("Auto-configured Vault merge store")
		}
		// Could also default to S3 if only AWS is available
	}
}

// getEnvOrDefault returns env var value or default
func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// AutoDetectAndConfigure performs full auto-detection and config application
func (c *Config) AutoDetectAndConfigure() DetectedClients {
	detected := AutoDetectClients()
	c.ApplyAutoDetection(detected)
	c.AutoConfigure()
	return detected
}
