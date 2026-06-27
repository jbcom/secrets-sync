package pipeline

import (
	"encoding/json"
	"testing"
)

func TestRuntimeAuthApplyToConfigUsesCallerSessionMaterial(t *testing.T) {
	cfg := &Config{}
	auth := &RuntimeAuth{
		Vault: &VaultRuntimeAuth{
			Address:   "https://vault.example.test",
			Namespace: "platform",
			Token:     "vault-token",
		},
		AWS: &AWSRuntimeAuth{
			Region:          "us-west-2",
			AccessKeyID:     "AKIAEXAMPLE",
			SecretAccessKey: "secret",
			SessionToken:    "session",
		},
	}

	auth.applyToConfig(cfg)

	if cfg.Vault.Address != auth.Vault.Address {
		t.Fatalf("Vault address = %q, want %q", cfg.Vault.Address, auth.Vault.Address)
	}
	if cfg.Vault.Namespace != auth.Vault.Namespace {
		t.Fatalf("Vault namespace = %q, want %q", cfg.Vault.Namespace, auth.Vault.Namespace)
	}
	if cfg.Vault.Auth.Token == nil || cfg.Vault.Auth.Token.Token != auth.Vault.Token {
		t.Fatalf("Vault token was not applied to runtime config")
	}
	if cfg.AWS.Region != auth.AWS.Region {
		t.Fatalf("AWS region = %q, want %q", cfg.AWS.Region, auth.AWS.Region)
	}
}

func TestRuntimeAuthDelegateModeDoesNotOverrideConfig(t *testing.T) {
	cfg := &Config{
		Vault: VaultConfig{
			Address:   "https://configured-vault.example.test",
			Namespace: "configured",
		},
		AWS: AWSConfig{Region: "us-east-1"},
	}
	auth := &RuntimeAuth{
		DelegateAuth: true,
		Vault: &VaultRuntimeAuth{
			Address:   "https://runtime-vault.example.test",
			Namespace: "runtime",
			Token:     "runtime-token",
		},
		AWS: &AWSRuntimeAuth{Region: "us-west-2"},
	}

	auth.applyToConfig(cfg)

	if cfg.Vault.Address != "https://configured-vault.example.test" {
		t.Fatalf("delegate mode should not override Vault address, got %q", cfg.Vault.Address)
	}
	if cfg.Vault.Namespace != "configured" {
		t.Fatalf("delegate mode should not override Vault namespace, got %q", cfg.Vault.Namespace)
	}
	if cfg.Vault.Auth.Token != nil {
		t.Fatalf("delegate mode should not inject Vault token")
	}
	if cfg.AWS.Region != "us-east-1" {
		t.Fatalf("delegate mode should not override AWS region, got %q", cfg.AWS.Region)
	}
}

func TestRuntimeAuthSecretsAreNotSerialized(t *testing.T) {
	auth := &RuntimeAuth{
		Vault: &VaultRuntimeAuth{Token: "vault-token"},
		AWS: &AWSRuntimeAuth{
			AccessKeyID:     "AKIAEXAMPLE",
			SecretAccessKey: "secret",
			SessionToken:    "session",
		},
	}

	encoded, err := json.Marshal(auth)
	if err != nil {
		t.Fatalf("json.Marshal(RuntimeAuth) failed: %v", err)
	}
	if string(encoded) != "{}" {
		t.Fatalf("RuntimeAuth JSON = %s, want {}", encoded)
	}
}

func TestPipelineRuntimeClientsUseSuppliedSessionMaterial(t *testing.T) {
	p := &Pipeline{
		config: &Config{
			Vault: VaultConfig{
				Address:   "https://configured-vault.example.test",
				Namespace: "configured",
				Auth: VaultAuthConfig{
					Token: &TokenAuth{Token: "configured-token"},
				},
			},
			AWS: AWSConfig{Region: "us-east-1"},
		},
		runtimeAuth: &RuntimeAuth{
			Vault: &VaultRuntimeAuth{
				Address:   "https://runtime-vault.example.test",
				Namespace: "runtime",
				Token:     "runtime-token",
			},
			AWS: &AWSRuntimeAuth{
				Region:          "us-west-2",
				AccessKeyID:     "AKIAEXAMPLE",
				SecretAccessKey: "secret",
				SessionToken:    "session",
				EndpointURL:     "http://localhost:4566",
			},
		},
	}

	vaultClient := p.vaultClient("secret/path")
	if vaultClient.Address != "https://runtime-vault.example.test" {
		t.Fatalf("Vault client address = %q", vaultClient.Address)
	}
	if vaultClient.Namespace != "runtime" {
		t.Fatalf("Vault client namespace = %q", vaultClient.Namespace)
	}
	if vaultClient.Token != "runtime-token" {
		t.Fatalf("Vault client token was not set from runtime auth")
	}

	awsClient := p.awsClient("", "", "target")
	if awsClient.Region != "us-west-2" {
		t.Fatalf("AWS client region = %q", awsClient.Region)
	}
	if awsClient.RuntimeAccessKeyID != "AKIAEXAMPLE" {
		t.Fatalf("AWS client access key was not set from runtime auth")
	}
	if awsClient.RuntimeSecretAccessKey != "secret" {
		t.Fatalf("AWS client secret access key was not set from runtime auth")
	}
	if awsClient.RuntimeSessionToken != "session" {
		t.Fatalf("AWS client session token was not set from runtime auth")
	}
	if awsClient.Endpoint != "http://localhost:4566" {
		t.Fatalf("AWS client endpoint = %q", awsClient.Endpoint)
	}
}
