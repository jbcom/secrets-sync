package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/extended-data-library/secretssync/pkg/client/aws"
	"github.com/extended-data-library/secretssync/pkg/client/vault"
	log "github.com/sirupsen/logrus"
)

// fetchVaultSecrets fetches all secrets from a Vault path
func (p *Pipeline) fetchVaultSecrets(ctx context.Context, path string) (map[string]interface{}, error) {
	l := log.WithFields(log.Fields{
		"action": "fetchVaultSecrets",
		"path":   path,
	})

	vaultClient := &vault.VaultClient{
		Address:                  p.config.Vault.Address,
		Namespace:                p.config.Vault.Namespace,
		Path:                     path,
		MaxTraversalDepth:        p.config.Vault.MaxTraversalDepth,
		MaxSecretsPerMount:       p.config.Vault.MaxSecretsPerMount,
		QueueCompactionThreshold: p.config.Vault.QueueCompactionThreshold,
	}

	if err := vaultClient.Init(ctx); err != nil {
		l.WithError(err).Debug("Failed to initialize Vault client")
		return nil, err
	}
	defer vaultClient.Close()

	secretsList, err := vaultClient.ListSecrets(ctx, path)
	if err != nil {
		l.WithError(err).Debug("Failed to list secrets")
		return map[string]interface{}{}, nil
	}

	secrets := make(map[string]interface{})
	for _, secretName := range secretsList {
		secretPath := fmt.Sprintf("%s/%s", path, secretName)
		secretData, err := vaultClient.GetSecret(ctx, secretPath)
		if err != nil {
			l.WithError(err).WithField("secretPath", secretPath).Debug("Failed to get secret")
			continue
		}

		var data interface{}
		if err := json.Unmarshal(secretData, &data); err != nil {
			l.WithError(err).WithField("secretPath", secretPath).Debug("Failed to parse secret")
			continue
		}
		secrets[secretName] = data
	}

	return secrets, nil
}

// fetchAWSSecrets fetches all secrets from AWS Secrets Manager
func (p *Pipeline) fetchAWSSecrets(ctx context.Context, roleARN, region string) (map[string]interface{}, error) {
	l := log.WithFields(log.Fields{
		"action":  "fetchAWSSecrets",
		"roleARN": roleARN,
		"region":  region,
	})

	awsClient := &aws.AwsClient{
		RoleArn: roleARN,
		Region:  region,
		Name:    "fetch-current-state",
	}

	if err := awsClient.Init(ctx); err != nil {
		l.WithError(err).Debug("Failed to initialize AWS client")
		return nil, err
	}

	secretsList, err := awsClient.ListSecrets(ctx, "")
	if err != nil {
		l.WithError(err).Debug("Failed to list AWS secrets")
		return map[string]interface{}{}, nil
	}

	secrets := make(map[string]interface{})
	for _, secretName := range secretsList {
		secretData, err := awsClient.GetSecret(ctx, secretName)
		if err != nil {
			l.WithError(err).WithField("secretName", secretName).Debug("Failed to get secret")
			continue
		}

		var data interface{}
		if err := json.Unmarshal(secretData, &data); err != nil {
			l.WithError(err).WithField("secretName", secretName).Debug("Failed to parse secret")
			continue
		}
		secrets[secretName] = data
	}

	return secrets, nil
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
