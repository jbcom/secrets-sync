package pipeline

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jbcom/secrets-sync/pkg/driver"
	log "github.com/sirupsen/logrus"
)

// fetchSourceSecrets reads every secret beneath path from any registered source
// backend, returning a name→value map. It is the driver-generic fetch path that
// new providers route through; the Vault- and AWS-specific helpers below remain
// for the paths that still need provider-specific construction/auth.
func (p *Pipeline) fetchSourceSecrets(ctx context.Context, src driver.SourceBackend, path string) (map[string]interface{}, error) {
	if err := src.Init(ctx); err != nil {
		return nil, err
	}
	defer src.Close()

	names, err := src.ListSecrets(ctx, path)
	if err != nil {
		return map[string]interface{}{}, nil
	}

	// A trailing-slash boundary is required so that a scope of "kv/app" does not
	// match a sibling secret under "kv/application" (which would mangle the key).
	var prefix string
	if path != "" {
		prefix = strings.TrimRight(path, "/") + "/"
	}

	secrets := make(map[string]interface{}, len(names))
	for _, name := range names {
		// Backends like Vault return fully-qualified names already carrying the
		// scope prefix; flat backends like AWS return bare names. Read with the
		// fully-qualified path, key by the scope-relative name.
		secretPath := name
		hasPrefix := prefix != "" && strings.HasPrefix(name, prefix)
		if prefix != "" && !hasPrefix {
			secretPath = prefix + name
		}
		raw, err := src.GetSecret(ctx, secretPath)
		if err != nil {
			log.WithError(err).Debug("Failed to get secret from source backend")
			continue
		}
		var data interface{}
		if err := json.Unmarshal(raw, &data); err != nil {
			log.WithError(err).Debug("Failed to parse secret from source backend")
			continue
		}
		key := name
		if hasPrefix {
			key = strings.TrimPrefix(name, prefix)
		}
		secrets[key] = data
	}
	return secrets, nil
}

// fetchVaultSecrets fetches all secrets from a Vault path. The Vault client is
// constructed with full runtime auth via vaultClient(); reading is delegated to
// the driver-generic fetchSourceSecrets so every source backend shares one code
// path.
func (p *Pipeline) fetchVaultSecrets(ctx context.Context, path string) (map[string]interface{}, error) {
	return p.fetchSourceSecrets(ctx, p.vaultClient(path), path)
}

// fetchAWSSecrets fetches all secrets from AWS Secrets Manager. The AWS client
// is constructed with full cross-account role/region/runtime auth via
// awsClient(); reading is delegated to the driver-generic fetchSourceSecrets.
func (p *Pipeline) fetchAWSSecrets(ctx context.Context, roleARN, region string) (map[string]interface{}, error) {
	return p.fetchSourceSecrets(ctx, p.awsClient(roleARN, region, "fetch-current-state"), "")
}

// fetchS3MergeSecrets fetches all secrets from S3 merge store for a target
func (p *Pipeline) fetchS3MergeSecrets(ctx context.Context, targetName string) (map[string]interface{}, error) {
	l := log.WithFields(log.Fields{
		"action": "fetchS3MergeSecrets",
		"target": targetName,
	})

	if p.s3Store == nil {
		return map[string]interface{}{}, nil
	}

	secrets, err := p.s3Store.ListSecrets(ctx, targetName)
	if err != nil {
		l.WithError(err).Debug("Failed to list S3 secrets")
		return map[string]interface{}{}, nil
	}

	secretsMap := make(map[string]interface{})
	for _, secretName := range secrets {
		secretData, err := p.s3Store.ReadSecret(ctx, targetName, secretName)
		if err != nil {
			l.WithError(err).WithField("secretName", secretName).Debug("Failed to read secret")
			continue
		}
		secretsMap[secretName] = secretData
	}

	return secretsMap, nil
}
