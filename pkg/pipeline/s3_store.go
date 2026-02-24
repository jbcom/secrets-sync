// Package pipeline provides S3-based merge store implementation for secrets aggregation.
package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	log "github.com/sirupsen/logrus"
)

// SecretVersion represents a versioned secret with metadata (v1.2.0 - Requirement 24)
type SecretVersion struct {
	Path      string                 `json:"path"`
	Version   int                    `json:"version"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
	Hash      string                 `json:"hash,omitempty"`
}

// VersionStore interface for version management (v1.2.0 - Requirement 24)
type VersionStore interface {
	GetVersion(ctx context.Context, path string, version int) (*SecretVersion, error)
	ListVersions(ctx context.Context, path string) ([]SecretVersion, error)
	GetLatest(ctx context.Context, path string) (*SecretVersion, error)
	StoreVersion(ctx context.Context, secret *SecretVersion) error
}

// S3MergeStore implements a merge store using S3 for intermediate secret storage.
// This is useful when you want to use S3 as a central repository for merged secrets
// before syncing to target accounts, or for audit/backup purposes.
type S3MergeStore struct {
	Bucket   string
	Prefix   string
	KMSKeyID string
	Region   string

	// Version management (v1.2.0 - Requirement 24)
	VersioningEnabled bool
	RetainVersions    int

	client *s3.Client
}

// NewS3MergeStore creates a new S3-based merge store
func NewS3MergeStore(ctx context.Context, cfg *MergeStoreS3, region string) (*S3MergeStore, error) {
	l := log.WithFields(log.Fields{
		"action": "NewS3MergeStore",
		"bucket": cfg.Bucket,
		"prefix": cfg.Prefix,
	})
	l.Debug("Creating S3 merge store")

	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	store := &S3MergeStore{
		Bucket:   cfg.Bucket,
		Prefix:   cfg.Prefix,
		KMSKeyID: cfg.KMSKeyID,
		Region:   region,
		client:   s3.NewFromConfig(awsCfg),
	}

	// Configure versioning if enabled (v1.2.0 - Requirement 24)
	if cfg.Versioning != nil {
		store.VersioningEnabled = cfg.Versioning.Enabled
		store.RetainVersions = cfg.Versioning.RetainVersions
		if store.RetainVersions <= 0 {
			store.RetainVersions = 10 // Default retention
		}
		l.WithFields(log.Fields{
			"versioning_enabled": store.VersioningEnabled,
			"retain_versions":    store.RetainVersions,
		}).Debug("Versioning configured")
	}

	return store, nil
}

// keyPath returns the full S3 key for a given target and secret name
func (s *S3MergeStore) keyPath(targetName, secretName string) string {
	prefix := s.Prefix
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return fmt.Sprintf("%s%s/%s.json", prefix, targetName, secretName)
}

// WriteSecret writes a secret to S3
func (s *S3MergeStore) WriteSecret(ctx context.Context, targetName, secretName string, data map[string]interface{}) error {
	l := log.WithFields(log.Fields{
		"action":     "S3MergeStore.WriteSecret",
		"bucket":     s.Bucket,
		"target":     targetName,
		"secretName": secretName,
	})
	l.Debug("Writing secret to S3")

	key := s.keyPath(targetName, secretName)

	// Marshal secret data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal secret data: %w", err)
	}

	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(jsonData),
		ContentType: aws.String("application/json"),
	}

	// Use KMS encryption if configured
	if s.KMSKeyID != "" {
		input.ServerSideEncryption = "aws:kms"
		input.SSEKMSKeyId = aws.String(s.KMSKeyID)
	} else {
		input.ServerSideEncryption = "AES256"
	}

	_, err = s.client.PutObject(ctx, input)
	if err != nil {
		l.WithError(err).Error("Failed to write secret to S3")
		return fmt.Errorf("failed to put object: %w", err)
	}

	l.Debug("Successfully wrote secret to S3")
	return nil
}

// ReadSecret reads a secret from S3
func (s *S3MergeStore) ReadSecret(ctx context.Context, targetName, secretName string) (map[string]interface{}, error) {
	l := log.WithFields(log.Fields{
		"action":     "S3MergeStore.ReadSecret",
		"bucket":     s.Bucket,
		"target":     targetName,
		"secretName": secretName,
	})
	l.Debug("Reading secret from S3")

	key := s.keyPath(targetName, secretName)

	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer func() { _ = output.Body.Close() }()

	body, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secret: %w", err)
	}

	return data, nil
}

// ListSecrets lists all secrets for a target
func (s *S3MergeStore) ListSecrets(ctx context.Context, targetName string) ([]string, error) {
	l := log.WithFields(log.Fields{
		"action": "S3MergeStore.ListSecrets",
		"bucket": s.Bucket,
		"target": targetName,
	})
	l.Debug("Listing secrets from S3")

	prefix := s.Prefix
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	targetPrefix := fmt.Sprintf("%s%s/", prefix, targetName)

	var secrets []string
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.Bucket),
		Prefix: aws.String(targetPrefix),
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range output.Contents {
			key := aws.ToString(obj.Key)
			// Extract secret name from key (remove prefix and .json suffix)
			name := strings.TrimPrefix(key, targetPrefix)
			name = strings.TrimSuffix(name, ".json")
			if name != "" && !strings.Contains(name, "/") {
				secrets = append(secrets, name)
			}
		}
	}

	return secrets, nil
}

// DeleteSecret deletes a secret from S3
func (s *S3MergeStore) DeleteSecret(ctx context.Context, targetName, secretName string) error {
	l := log.WithFields(log.Fields{
		"action":     "S3MergeStore.DeleteSecret",
		"bucket":     s.Bucket,
		"target":     targetName,
		"secretName": secretName,
	})
	l.Debug("Deleting secret from S3")

	key := s.keyPath(targetName, secretName)

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// GetMergePath returns the S3 "path" representation for a target
// This is used for logging and reporting purposes
func (s *S3MergeStore) GetMergePath(targetName string) string {
	prefix := s.Prefix
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return fmt.Sprintf("s3://%s/%s%s", s.Bucket, prefix, targetName)
}

// GetBundlePath returns the S3 path for a specific bundle
func (s *S3MergeStore) GetBundlePath(targetName, bundleID string) string {
	prefix := s.Prefix
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return fmt.Sprintf("s3://%s/%sbundles/%s/%s", s.Bucket, prefix, targetName, bundleID)
}

// bundleKey returns the S3 key for a bundle
func (s *S3MergeStore) bundleKey(targetName, bundleID string) string {
	prefix := s.Prefix
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return fmt.Sprintf("%sbundles/%s/%s.json", prefix, targetName, bundleID)
}

// WriteMergedBundle writes a complete merged bundle to S3 as a single JSON blob
func (s *S3MergeStore) WriteMergedBundle(ctx context.Context, targetName, bundleID string, secrets map[string]interface{}) error {
	l := log.WithFields(log.Fields{
		"action":   "S3MergeStore.WriteMergedBundle",
		"bucket":   s.Bucket,
		"target":   targetName,
		"bundleID": bundleID,
	})
	l.Debug("Writing merged bundle to S3")

	key := s.bundleKey(targetName, bundleID)

	// Marshal all secrets to JSON
	jsonData, err := json.Marshal(secrets)
	if err != nil {
		return fmt.Errorf("failed to marshal bundle: %w", err)
	}

	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(jsonData),
		ContentType: aws.String("application/json"),
	}

	// Use KMS encryption if configured
	if s.KMSKeyID != "" {
		input.ServerSideEncryption = "aws:kms"
		input.SSEKMSKeyId = aws.String(s.KMSKeyID)
	} else {
		input.ServerSideEncryption = "AES256"
	}

	_, err = s.client.PutObject(ctx, input)
	if err != nil {
		l.WithError(err).Error("Failed to write bundle to S3")
		return fmt.Errorf("failed to put object: %w", err)
	}

	l.WithField("secretsCount", len(secrets)).Debug("Successfully wrote bundle to S3")
	return nil
}

// ReadMergedBundle reads a complete merged bundle from S3
func (s *S3MergeStore) ReadMergedBundle(ctx context.Context, targetName, bundleID string) (map[string]map[string]interface{}, error) {
	l := log.WithFields(log.Fields{
		"action":   "S3MergeStore.ReadMergedBundle",
		"bucket":   s.Bucket,
		"target":   targetName,
		"bundleID": bundleID,
	})
	l.Debug("Reading merged bundle from S3")

	key := s.bundleKey(targetName, bundleID)

	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer func() { _ = output.Body.Close() }()

	body, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	// The bundle is stored as map[string]interface{} but we need map[string]map[string]interface{}
	var rawData map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bundle: %w", err)
	}

	// Convert to expected format
	result := make(map[string]map[string]interface{})
	for k, v := range rawData {
		if m, ok := v.(map[string]interface{}); ok {
			result[k] = m
		}
	}

	return result, nil
}

// DeleteBundle deletes a bundle from S3
func (s *S3MergeStore) DeleteBundle(ctx context.Context, targetName, bundleID string) error {
	l := log.WithFields(log.Fields{
		"action":   "S3MergeStore.DeleteBundle",
		"bucket":   s.Bucket,
		"target":   targetName,
		"bundleID": bundleID,
	})
	l.Debug("Deleting bundle from S3")

	key := s.bundleKey(targetName, bundleID)

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// Version management methods (v1.2.0 - Requirement 24)

// versionKeyPath returns the S3 key for a specific version of a secret
func (s *S3MergeStore) versionKeyPath(targetName, secretName string, version int) string {
	prefix := s.Prefix
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return fmt.Sprintf("%sversions/%s/%s/v%d.json", prefix, targetName, secretName, version)
}

// versionMetadataKeyPath returns the S3 key for version metadata
func (s *S3MergeStore) versionMetadataKeyPath(targetName, secretName string) string {
	prefix := s.Prefix
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return fmt.Sprintf("%sversions/%s/%s/metadata.json", prefix, targetName, secretName)
}

// GetVersion retrieves a specific version of a secret
func (s *S3MergeStore) GetVersion(ctx context.Context, path string, version int) (*SecretVersion, error) {
	if !s.VersioningEnabled {
		return nil, fmt.Errorf("versioning not enabled")
	}

	// Parse target and secret name from path
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid path format: %s", path)
	}
	targetName, secretName := parts[0], parts[1]

	l := log.WithFields(log.Fields{
		"action":     "S3MergeStore.GetVersion",
		"bucket":     s.Bucket,
		"target":     targetName,
		"secretName": secretName,
		"version":    version,
	})
	l.Debug("Getting specific version from S3")

	key := s.versionKeyPath(targetName, secretName, version)

	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get version %d: %w", version, err)
	}
	defer func() { _ = output.Body.Close() }()

	body, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read version body: %w", err)
	}

	var secretVersion SecretVersion
	if err := json.Unmarshal(body, &secretVersion); err != nil {
		return nil, fmt.Errorf("failed to unmarshal version: %w", err)
	}

	return &secretVersion, nil
}

// ListVersions lists all versions of a secret
func (s *S3MergeStore) ListVersions(ctx context.Context, path string) ([]SecretVersion, error) {
	if !s.VersioningEnabled {
		return nil, fmt.Errorf("versioning not enabled")
	}

	// Parse target and secret name from path
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid path format: %s", path)
	}
	targetName, secretName := parts[0], parts[1]

	l := log.WithFields(log.Fields{
		"action":     "S3MergeStore.ListVersions",
		"bucket":     s.Bucket,
		"target":     targetName,
		"secretName": secretName,
	})
	l.Debug("Listing versions from S3")

	prefix := s.Prefix
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	versionPrefix := fmt.Sprintf("%sversions/%s/%s/", prefix, targetName, secretName)

	var versions []SecretVersion
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.Bucket),
		Prefix: aws.String(versionPrefix),
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list version objects: %w", err)
		}

		for _, obj := range output.Contents {
			key := aws.ToString(obj.Key)
			// Skip metadata files
			if strings.HasSuffix(key, "metadata.json") {
				continue
			}

			// Get the version object
			versionOutput, err := s.client.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(s.Bucket),
				Key:    aws.String(key),
			})
			if err != nil {
				l.WithError(err).WithField("key", key).Warn("Failed to get version object")
				continue
			}

			body, err := io.ReadAll(versionOutput.Body)
			_ = versionOutput.Body.Close()
			if err != nil {
				l.WithError(err).WithField("key", key).Warn("Failed to read version body")
				continue
			}

			var version SecretVersion
			if err := json.Unmarshal(body, &version); err != nil {
				l.WithError(err).WithField("key", key).Warn("Failed to unmarshal version")
				continue
			}

			versions = append(versions, version)
		}
	}

	// Sort by version number (descending)
	for i := 0; i < len(versions)-1; i++ {
		for j := i + 1; j < len(versions); j++ {
			if versions[i].Version < versions[j].Version {
				versions[i], versions[j] = versions[j], versions[i]
			}
		}
	}

	return versions, nil
}

// GetLatest retrieves the latest version of a secret
func (s *S3MergeStore) GetLatest(ctx context.Context, path string) (*SecretVersion, error) {
	if !s.VersioningEnabled {
		return nil, fmt.Errorf("versioning not enabled")
	}

	versions, err := s.ListVersions(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions found for path: %s", path)
	}

	return &versions[0], nil
}

// StoreVersion stores a new version of a secret
func (s *S3MergeStore) StoreVersion(ctx context.Context, secret *SecretVersion) error {
	if !s.VersioningEnabled {
		return fmt.Errorf("versioning not enabled")
	}

	// Parse target and secret name from path
	parts := strings.SplitN(secret.Path, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid path format: %s", secret.Path)
	}
	targetName, secretName := parts[0], parts[1]

	l := log.WithFields(log.Fields{
		"action":     "S3MergeStore.StoreVersion",
		"bucket":     s.Bucket,
		"target":     targetName,
		"secretName": secretName,
		"version":    secret.Version,
	})
	l.Debug("Storing version to S3")

	// Set timestamp if not provided
	if secret.Timestamp.IsZero() {
		secret.Timestamp = time.Now().UTC()
	}

	key := s.versionKeyPath(targetName, secretName, secret.Version)

	// Marshal version data to JSON
	jsonData, err := json.Marshal(secret)
	if err != nil {
		return fmt.Errorf("failed to marshal version data: %w", err)
	}

	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(jsonData),
		ContentType: aws.String("application/json"),
	}

	// Use KMS encryption if configured
	if s.KMSKeyID != "" {
		input.ServerSideEncryption = "aws:kms"
		input.SSEKMSKeyId = aws.String(s.KMSKeyID)
	} else {
		input.ServerSideEncryption = "AES256"
	}

	_, err = s.client.PutObject(ctx, input)
	if err != nil {
		l.WithError(err).Error("Failed to store version to S3")
		return fmt.Errorf("failed to put version object: %w", err)
	}

	// Clean up old versions if retention limit is set
	if s.RetainVersions > 0 {
		if err := s.cleanupOldVersions(ctx, targetName, secretName); err != nil {
			l.WithError(err).Warn("Failed to cleanup old versions")
		}
	}

	l.Debug("Successfully stored version to S3")
	return nil
}

// cleanupOldVersions removes versions beyond the retention limit
func (s *S3MergeStore) cleanupOldVersions(ctx context.Context, targetName, secretName string) error {
	path := fmt.Sprintf("%s/%s", targetName, secretName)
	versions, err := s.ListVersions(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to list versions for cleanup: %w", err)
	}

	if len(versions) <= s.RetainVersions {
		return nil // Nothing to clean up
	}

	// Delete versions beyond retention limit
	versionsToDelete := versions[s.RetainVersions:]
	for _, version := range versionsToDelete {
		key := s.versionKeyPath(targetName, secretName, version.Version)
		_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.Bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"target":     targetName,
				"secretName": secretName,
				"version":    version.Version,
			}).Warn("Failed to delete old version")
		}
	}

	return nil
}
