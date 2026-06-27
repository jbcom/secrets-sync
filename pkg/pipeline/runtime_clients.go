package pipeline

import (
	"github.com/jbcom/secrets-sync/pkg/client/aws"
	"github.com/jbcom/secrets-sync/pkg/client/vault"
)

func (p *Pipeline) runtimeVaultAuth() *VaultRuntimeAuth {
	if p == nil || p.runtimeAuth == nil || p.runtimeAuth.DelegateAuth {
		return nil
	}
	return p.runtimeAuth.Vault
}

func (p *Pipeline) runtimeAWSAuth() *AWSRuntimeAuth {
	if p == nil || p.runtimeAuth == nil || p.runtimeAuth.DelegateAuth {
		return nil
	}
	return p.runtimeAuth.AWS
}

func (p *Pipeline) vaultClient(path string) *vault.VaultClient {
	client := &vault.VaultClient{
		Address:                  p.config.Vault.Address,
		Namespace:                p.config.Vault.Namespace,
		Path:                     path,
		MaxTraversalDepth:        p.config.Vault.MaxTraversalDepth,
		MaxSecretsPerMount:       p.config.Vault.MaxSecretsPerMount,
		QueueCompactionThreshold: p.config.Vault.QueueCompactionThreshold,
	}

	if p.config.Vault.Auth.Token != nil {
		client.Token = p.config.Vault.Auth.Token.Token
	}
	if p.config.Vault.Auth.Kubernetes != nil {
		client.AuthMethod = p.config.Vault.Auth.Kubernetes.MountPath
		client.Role = p.config.Vault.Auth.Kubernetes.Role
	}
	if p.config.Vault.Auth.AppRole != nil {
		client.AuthMethod = p.config.Vault.Auth.AppRole.Mount
	}

	if auth := p.runtimeVaultAuth(); auth != nil {
		if auth.Address != "" {
			client.Address = auth.Address
		}
		if auth.Namespace != "" {
			client.Namespace = auth.Namespace
		}
		if auth.Token != "" {
			client.Token = auth.Token
		}
	}

	return client
}

func (p *Pipeline) awsClient(roleARN, region, name string) *aws.AwsClient {
	explicitRegion := region != ""
	if region == "" {
		region = p.config.AWS.Region
	}

	client := &aws.AwsClient{
		Name:       name,
		RoleArn:    roleARN,
		Region:     region,
		MaxRetries: p.config.AWS.MaxRetries,
		RetryMode:  p.config.AWS.RetryMode,
	}

	if auth := p.runtimeAWSAuth(); auth != nil {
		if auth.Region != "" && !explicitRegion {
			client.Region = auth.Region
		}
		if auth.RoleARN != "" && client.RoleArn == "" {
			client.RoleArn = auth.RoleARN
		}
		client.RuntimeAccessKeyID = auth.AccessKeyID
		client.RuntimeSecretAccessKey = auth.SecretAccessKey
		client.RuntimeSessionToken = auth.SessionToken
		client.Endpoint = auth.EndpointURL
	}

	return client
}
