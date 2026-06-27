package main

import (
	"context"
	"os"
	"testing"

	"github.com/jbcom/secrets-sync/pkg/diff"
	"github.com/jbcom/secrets-sync/pkg/pipeline"
)

func TestRuntimeAuthMapsProviderSession(t *testing.T) {
	auth := runtimeAuth(providerSession{
		VaultAddress:       "https://vault.example.test",
		VaultNamespace:     "platform",
		VaultToken:         "vault-token",
		AWSRegion:          "us-west-2",
		AWSAccessKeyID:     "AKIAEXAMPLE",
		AWSSecretAccessKey: "secret",
		AWSSessionToken:    "session",
		AWSRoleARN:         "arn:aws:iam::123456789012:role/example",
		AWSEndpointURL:     "http://localhost:4566",
	})

	if auth == nil {
		t.Fatal("runtimeAuth returned nil")
	}
	if auth.Vault == nil || auth.Vault.Token != "vault-token" {
		t.Fatalf("Vault runtime auth not mapped: %#v", auth.Vault)
	}
	if auth.AWS == nil || auth.AWS.Region != "us-west-2" {
		t.Fatalf("AWS runtime auth not mapped: %#v", auth.AWS)
	}
}

func TestPipelineOptionsDefaultsToFullPipeline(t *testing.T) {
	opts := pipelineOptions(requestOptions{
		DryRun:       true,
		Targets:      "prod,staging",
		OutputFormat: "json",
	})

	if opts.Operation != pipeline.OperationPipeline {
		t.Fatalf("Operation = %q, want %q", opts.Operation, pipeline.OperationPipeline)
	}
	if !opts.DryRun || !opts.ComputeDiff {
		t.Fatalf("dry-run options should enable dry run and diff: %#v", opts)
	}
	if len(opts.Targets) != 2 || opts.Targets[0] != "prod" || opts.Targets[1] != "staging" {
		t.Fatalf("targets = %#v", opts.Targets)
	}
	if opts.OutputFormat != diff.OutputFormatJSON {
		t.Fatalf("OutputFormat = %q, want %q", opts.OutputFormat, diff.OutputFormatJSON)
	}
}

func TestResolveConfigWritesInlineYAMLToTempFile(t *testing.T) {
	path, cleanup, err := resolveConfig(context.Background(), request{
		ConfigYAML: "targets:\n  prod:\n    imports: []\n",
	})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("resolveConfig failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read temp config: %v", err)
	}
	if string(data) != "targets:\n  prod:\n    imports: []\n" {
		t.Fatalf("temp config contents = %q", data)
	}
}
