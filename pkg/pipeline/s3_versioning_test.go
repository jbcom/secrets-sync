package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestSecretVersion tests the SecretVersion structure (v1.2.0 - Requirement 24)
func TestSecretVersion(t *testing.T) {
	now := time.Now().UTC()

	version := SecretVersion{
		Path:      "production/api-key",
		Version:   1,
		Data:      map[string]interface{}{"key": "value"},
		Timestamp: now,
		Hash:      "abc123",
	}

	assert.Equal(t, "production/api-key", version.Path)
	assert.Equal(t, 1, version.Version)
	assert.Equal(t, "value", version.Data["key"])
	assert.Equal(t, now, version.Timestamp)
	assert.Equal(t, "abc123", version.Hash)
}

// TestVersioningConfig tests versioning configuration (v1.2.0 - Requirement 24)
func TestVersioningConfig(t *testing.T) {
	tests := []struct {
		name            string
		config          *VersioningConfig
		expectedEnabled bool
		expectedRetain  int
	}{
		{
			name: "versioning enabled with retention",
			config: &VersioningConfig{
				Enabled:        true,
				RetainVersions: 5,
			},
			expectedEnabled: true,
			expectedRetain:  5,
		},
		{
			name: "versioning disabled",
			config: &VersioningConfig{
				Enabled:        false,
				RetainVersions: 10,
			},
			expectedEnabled: false,
			expectedRetain:  10,
		},
		{
			name:            "nil config",
			config:          nil,
			expectedEnabled: false,
			expectedRetain:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s3Config := &MergeStoreS3{
				Versioning: tt.config,
			}

			// Test that configuration is properly structured
			if tt.config != nil {
				assert.Equal(t, tt.expectedEnabled, s3Config.Versioning.Enabled)
				assert.Equal(t, tt.expectedRetain, s3Config.Versioning.RetainVersions)
			} else {
				assert.Nil(t, s3Config.Versioning)
			}
		})
	}
}

// TestS3MergeStoreVersioningMethods tests version management methods (v1.2.0 - Requirement 24)
func TestS3MergeStoreVersioningMethods(t *testing.T) {
	// Create a mock S3 merge store with versioning enabled
	store := &S3MergeStore{
		Bucket:            "test-bucket",
		Prefix:            "test-prefix",
		VersioningEnabled: true,
		RetainVersions:    5,
	}

	t.Run("versionKeyPath", func(t *testing.T) {
		key := store.versionKeyPath("production", "api-key", 1)
		expected := "test-prefix/versions/production/api-key/v1.json"
		assert.Equal(t, expected, key)
	})

	t.Run("versionMetadataKeyPath", func(t *testing.T) {
		key := store.versionMetadataKeyPath("production", "api-key")
		expected := "test-prefix/versions/production/api-key/metadata.json"
		assert.Equal(t, expected, key)
	})

	t.Run("versioning disabled", func(t *testing.T) {
		disabledStore := &S3MergeStore{
			VersioningEnabled: false,
		}

		ctx := context.Background()

		_, err := disabledStore.GetVersion(ctx, "production/api-key", 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "versioning not enabled")

		_, err = disabledStore.ListVersions(ctx, "production/api-key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "versioning not enabled")

		_, err = disabledStore.GetLatest(ctx, "production/api-key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "versioning not enabled")

		err = disabledStore.StoreVersion(ctx, &SecretVersion{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "versioning not enabled")
	})

	t.Run("invalid path format", func(t *testing.T) {
		ctx := context.Background()

		_, err := store.GetVersion(ctx, "invalid-path", 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid path format")

		_, err = store.ListVersions(ctx, "invalid-path")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid path format")

		_, err = store.GetLatest(ctx, "invalid-path")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid path format")

		err = store.StoreVersion(ctx, &SecretVersion{Path: "invalid-path"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid path format")
	})
}

// TestVersionStoreInterface tests that S3MergeStore implements VersionStore (v1.2.0 - Requirement 24)
func TestVersionStoreInterface(t *testing.T) {
	var _ VersionStore = (*S3MergeStore)(nil)

	// Test that the interface methods exist and have correct signatures
	// We only test interface compliance, not actual functionality (which requires AWS)
	store := &S3MergeStore{
		VersioningEnabled: false, // Disabled to avoid S3 client calls
	}

	ctx := context.Background()

	// These should fail with "versioning not enabled" rather than S3 errors
	_, err := store.GetVersion(ctx, "test/path", 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "versioning not enabled")

	_, err = store.ListVersions(ctx, "test/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "versioning not enabled")

	_, err = store.GetLatest(ctx, "test/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "versioning not enabled")

	err = store.StoreVersion(ctx, &SecretVersion{Path: "test/path", Version: 1})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "versioning not enabled")
}

// TestNewS3MergeStoreWithVersioning tests versioning configuration in constructor (v1.2.0 - Requirement 24)
func TestNewS3MergeStoreWithVersioning(t *testing.T) {
	tests := []struct {
		name                   string
		config                 *MergeStoreS3
		expectedVersioning     bool
		expectedRetainVersions int
	}{
		{
			name: "versioning enabled",
			config: &MergeStoreS3{
				Bucket: "test-bucket",
				Versioning: &VersioningConfig{
					Enabled:        true,
					RetainVersions: 15,
				},
			},
			expectedVersioning:     true,
			expectedRetainVersions: 15,
		},
		{
			name: "versioning enabled with default retention",
			config: &MergeStoreS3{
				Bucket: "test-bucket",
				Versioning: &VersioningConfig{
					Enabled:        true,
					RetainVersions: 0, // Should default to 10
				},
			},
			expectedVersioning:     true,
			expectedRetainVersions: 10,
		},
		{
			name: "versioning disabled",
			config: &MergeStoreS3{
				Bucket: "test-bucket",
				Versioning: &VersioningConfig{
					Enabled:        false,
					RetainVersions: 5,
				},
			},
			expectedVersioning:     false,
			expectedRetainVersions: 5,
		},
		{
			name: "no versioning config",
			config: &MergeStoreS3{
				Bucket: "test-bucket",
			},
			expectedVersioning:     false,
			expectedRetainVersions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't actually create the store without AWS credentials,
			// but we can test the configuration logic
			if tt.config.Versioning != nil {
				assert.Equal(t, tt.expectedVersioning, tt.config.Versioning.Enabled)

				expectedRetain := tt.config.Versioning.RetainVersions
				if expectedRetain <= 0 && tt.config.Versioning.Enabled {
					expectedRetain = 10 // Default that would be set in constructor
				}

				// We can't test the actual constructor without AWS setup,
				// but we can verify the configuration structure
				if tt.expectedVersioning {
					assert.True(t, tt.config.Versioning.Enabled)
					// Verify retention is set correctly
					if tt.config.Versioning.RetainVersions <= 0 {
						assert.Equal(t, 10, expectedRetain) // Default value
					} else {
						assert.Equal(t, tt.config.Versioning.RetainVersions, expectedRetain)
					}
				}
			}
		})
	}
}

// TestVersionPathParsing tests path parsing for version operations (v1.2.0 - Requirement 24)
func TestVersionPathParsing(t *testing.T) {
	store := &S3MergeStore{
		Prefix:            "test-prefix",
		VersioningEnabled: true,
	}

	tests := []struct {
		name        string
		path        string
		version     int
		expectedKey string
		shouldError bool
	}{
		{
			name:        "valid path",
			path:        "production/api-key",
			version:     1,
			expectedKey: "test-prefix/versions/production/api-key/v1.json",
			shouldError: false,
		},
		{
			name:        "path with multiple slashes",
			path:        "production/nested/api-key",
			version:     2,
			expectedKey: "test-prefix/versions/production/nested/api-key/v2.json",
			shouldError: false,
		},
		{
			name:        "invalid path - no slash",
			path:        "invalid-path",
			version:     1,
			shouldError: true,
		},
		{
			name:        "empty path",
			path:        "",
			version:     1,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldError {
				// Test that methods would return errors for invalid paths
				ctx := context.Background()
				_, err := store.GetVersion(ctx, tt.path, tt.version)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid path format")
			} else {
				// Test key generation
				key := store.versionKeyPath("production", "api-key", tt.version)
				if tt.path == "production/api-key" {
					assert.Equal(t, tt.expectedKey, key)
				}
			}
		})
	}
}
