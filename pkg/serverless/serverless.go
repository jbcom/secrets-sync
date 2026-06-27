// Package serverless holds the provider-agnostic core of the secrets-sync
// serverless entrypoints. The AWS Lambda and Azure Functions binaries are thin
// adapters that decode their platform's trigger into a Request and call Handle.
package serverless

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	awscredentials "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jbcom/secrets-sync/pkg/diff"
	"github.com/jbcom/secrets-sync/pkg/pipeline"
)

// Request is the trigger payload for a serverless pipeline run.
type Request struct {
	ConfigYAML     string          `json:"config_yaml,omitempty"`
	ConfigPath     string          `json:"config_path,omitempty"`
	ConfigS3Bucket string          `json:"config_s3_bucket,omitempty"`
	ConfigS3Key    string          `json:"config_s3_key,omitempty"`
	Options        RequestOptions  `json:"options,omitempty"`
	Session        ProviderSession `json:"session,omitempty"`
}

// RequestOptions controls pipeline execution.
type RequestOptions struct {
	Operation       string `json:"operation,omitempty"`
	Targets         string `json:"targets,omitempty"`
	DryRun          bool   `json:"dry_run,omitempty"`
	ContinueOnError bool   `json:"continue_on_error,omitempty"`
	Parallelism     int    `json:"parallelism,omitempty"`
	ComputeDiff     bool   `json:"compute_diff,omitempty"`
	OutputFormat    string `json:"output_format,omitempty"`
	ShowValues      bool   `json:"show_values,omitempty"`
}

// ProviderSession carries runtime auth material supplied by the caller.
type ProviderSession struct {
	DelegateAuth       bool   `json:"delegate_auth,omitempty"`
	VaultAddress       string `json:"vault_address,omitempty"`
	VaultNamespace     string `json:"vault_namespace,omitempty"`
	VaultToken         string `json:"vault_token,omitempty"`
	AWSRegion          string `json:"aws_region,omitempty"`
	AWSAccessKeyID     string `json:"aws_access_key_id,omitempty"`
	AWSSecretAccessKey string `json:"aws_secret_access_key,omitempty"`
	AWSSessionToken    string `json:"aws_session_token,omitempty"`
	AWSRoleARN         string `json:"aws_role_arn,omitempty"`
	AWSEndpointURL     string `json:"aws_endpoint_url,omitempty"`
}

// Response is the serverless run summary.
type Response struct {
	Success          bool         `json:"success"`
	TargetCount      int          `json:"target_count"`
	SecretsProcessed int          `json:"secrets_processed"`
	SecretsAdded     int          `json:"secrets_added"`
	SecretsModified  int          `json:"secrets_modified"`
	SecretsRemoved   int          `json:"secrets_removed"`
	SecretsUnchanged int          `json:"secrets_unchanged"`
	DurationMs       int64        `json:"duration_ms"`
	ErrorMessage     string       `json:"error_message,omitempty"`
	Results          []ResultItem `json:"results"`
	DiffOutput       string       `json:"diff_output,omitempty"`
}

// ResultItem is a per-target result in the response.
type ResultItem struct {
	Target     string                 `json:"target"`
	Phase      string                 `json:"phase"`
	Operation  string                 `json:"operation"`
	Success    bool                   `json:"success"`
	Error      string                 `json:"error,omitempty"`
	DurationMs int64                  `json:"duration_ms"`
	Details    pipeline.ResultDetails `json:"details,omitempty"`
}

// Handle runs the pipeline for a Request and returns a Response. It never
// returns an error; failures are reported in Response.Success/ErrorMessage so
// every platform adapter can serialize a uniform body.
func Handle(ctx context.Context, event Request) Response {
	start := time.Now()

	cfgPath, cleanup, err := resolveConfig(ctx, event)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return Response{Success: false, ErrorMessage: err.Error()}
	}

	cfg, err := pipeline.LoadConfig(cfgPath)
	if err != nil {
		return Response{Success: false, ErrorMessage: err.Error()}
	}

	p, err := pipeline.NewWithContextAndRuntimeAuth(ctx, cfg, runtimeAuth(event.Session))
	if err != nil {
		return Response{Success: false, ErrorMessage: err.Error()}
	}

	opts, err := pipelineOptions(event.Options)
	if err != nil {
		return Response{Success: false, ErrorMessage: err.Error()}
	}
	results, runErr := p.Run(ctx, opts)
	duration := time.Since(start)

	resp := summarize(results, runErr, duration)
	if opts.ComputeDiff && p.Diff() != nil {
		resp.DiffOutput = diff.FormatDiffWithOptions(p.Diff(), opts.OutputFormat, event.Options.ShowValues)
	}
	return resp
}

// MarshalResponse JSON-encodes a Response, returning "{}" on the (unreachable)
// marshal error so adapters always have a body.
func MarshalResponse(resp Response) string {
	encoded, err := json.Marshal(resp)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func resolveConfig(ctx context.Context, event Request) (string, func(), error) {
	switch {
	case strings.TrimSpace(event.ConfigYAML) != "":
		return writeTempConfig(event.ConfigYAML)
	case event.ConfigS3Bucket != "" && event.ConfigS3Key != "":
		data, err := readS3Config(ctx, event.ConfigS3Bucket, event.ConfigS3Key, event.Session)
		if err != nil {
			return "", nil, err
		}
		return writeTempConfig(string(data))
	case event.ConfigPath != "":
		return event.ConfigPath, nil, nil
	default:
		if envPath := os.Getenv("SECRETS_SYNC_CONFIG"); envPath != "" {
			return envPath, nil, nil
		}
		return "", nil, fmt.Errorf("config_yaml, config_path, config_s3_bucket/config_s3_key, or SECRETS_SYNC_CONFIG is required")
	}
}

func writeTempConfig(contents string) (string, func(), error) {
	file, err := os.CreateTemp("", "secrets-sync-*.yaml")
	if err != nil {
		return "", nil, fmt.Errorf("create temp config: %w", err)
	}
	path := file.Name()
	cleanup := func() { _ = os.Remove(path) }
	if _, err := file.WriteString(contents); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, fmt.Errorf("write temp config: %w", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("secure temp config: %w", err)
	}
	return path, cleanup, nil
}

func readS3Config(ctx context.Context, bucket, key string, session ProviderSession) ([]byte, error) {
	loadOptions := []func(*config.LoadOptions) error{}
	if session.AWSRegion != "" {
		loadOptions = append(loadOptions, config.WithRegion(session.AWSRegion))
	}
	if !session.DelegateAuth && session.AWSAccessKeyID != "" && session.AWSSecretAccessKey != "" {
		loadOptions = append(loadOptions, config.WithCredentialsProvider(
			awscredentials.NewStaticCredentialsProvider(
				session.AWSAccessKeyID,
				session.AWSSecretAccessKey,
				session.AWSSessionToken,
			),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config for config_s3: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		if !session.DelegateAuth && session.AWSEndpointURL != "" {
			options.BaseEndpoint = aws.String(session.AWSEndpointURL)
		}
	})
	output, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("read config from s3://%s/%s: %w", bucket, key, err)
	}
	defer output.Body.Close()
	data, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, fmt.Errorf("read config body from s3://%s/%s: %w", bucket, key, err)
	}
	return data, nil
}

func runtimeAuth(session ProviderSession) *pipeline.RuntimeAuth {
	auth := &pipeline.RuntimeAuth{DelegateAuth: session.DelegateAuth}
	if auth.DelegateAuth {
		return auth
	}
	if session.VaultAddress != "" || session.VaultNamespace != "" || session.VaultToken != "" {
		auth.Vault = &pipeline.VaultRuntimeAuth{
			Address:   session.VaultAddress,
			Namespace: session.VaultNamespace,
			Token:     session.VaultToken,
		}
	}
	if session.AWSRegion != "" ||
		session.AWSAccessKeyID != "" ||
		session.AWSSecretAccessKey != "" ||
		session.AWSSessionToken != "" ||
		session.AWSRoleARN != "" ||
		session.AWSEndpointURL != "" {
		auth.AWS = &pipeline.AWSRuntimeAuth{
			Region:          session.AWSRegion,
			AccessKeyID:     session.AWSAccessKeyID,
			SecretAccessKey: session.AWSSecretAccessKey,
			SessionToken:    session.AWSSessionToken,
			RoleARN:         session.AWSRoleARN,
			EndpointURL:     session.AWSEndpointURL,
		}
	}
	if auth.DelegateAuth || auth.Vault != nil || auth.AWS != nil {
		return auth
	}
	return nil
}

func pipelineOptions(options RequestOptions) (pipeline.Options, error) {
	op := pipeline.OperationPipeline
	switch strings.ToLower(options.Operation) {
	case string(pipeline.OperationMerge):
		op = pipeline.OperationMerge
	case string(pipeline.OperationSync):
		op = pipeline.OperationSync
	case string(pipeline.OperationPipeline), "":
		op = pipeline.OperationPipeline
	default:
		return pipeline.Options{}, fmt.Errorf("unknown operation: %s", options.Operation)
	}

	return pipeline.Options{
		Operation:       op,
		Targets:         splitTargets(options.Targets),
		DryRun:          options.DryRun,
		ContinueOnError: options.ContinueOnError,
		Parallelism:     options.Parallelism,
		ComputeDiff:     options.ComputeDiff || options.DryRun,
		OutputFormat:    parseOutputFormat(options.OutputFormat),
	}, nil
}

func splitTargets(targets string) []string {
	if targets == "" {
		return nil
	}
	parts := strings.Split(targets, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parseOutputFormat(format string) diff.OutputFormat {
	switch strings.ToLower(format) {
	case "json":
		return diff.OutputFormatJSON
	case "github":
		return diff.OutputFormatGitHub
	case "compact":
		return diff.OutputFormatCompact
	case "side-by-side", "sidebyside", "side_by_side":
		return diff.OutputFormatSideBySide
	default:
		return diff.OutputFormatHuman
	}
}

func summarize(results []pipeline.Result, runErr error, duration time.Duration) Response {
	resp := Response{
		Success:    runErr == nil,
		DurationMs: duration.Milliseconds(),
		Results:    make([]ResultItem, 0, len(results)),
	}
	if runErr != nil {
		resp.ErrorMessage = runErr.Error()
	}

	targets := make(map[string]struct{})
	for _, result := range results {
		if result.Target != "" {
			targets[result.Target] = struct{}{}
		}
		resp.SecretsProcessed += result.Details.SecretsProcessed
		resp.SecretsAdded += result.Details.SecretsAdded
		resp.SecretsModified += result.Details.SecretsModified
		resp.SecretsRemoved += result.Details.SecretsRemoved
		resp.SecretsUnchanged += result.Details.SecretsUnchanged

		item := ResultItem{
			Target:     result.Target,
			Phase:      result.Phase,
			Operation:  result.Operation,
			Success:    result.Success,
			DurationMs: result.Duration.Milliseconds(),
			Details:    result.Details,
		}
		if result.Error != nil {
			item.Error = result.Error.Error()
		}
		resp.Results = append(resp.Results, item)
		if !result.Success {
			resp.Success = false
			if resp.ErrorMessage == "" {
				resp.ErrorMessage = item.Error
			}
		}
	}
	resp.TargetCount = len(targets)
	return resp
}
