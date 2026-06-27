package pipeline

// RuntimeAuth carries caller-owned provider authentication material into a
// pipeline run. It is intentionally excluded from YAML configuration and should
// be supplied by embedding packages, bindings, or in-process callers.
type RuntimeAuth struct {
	DelegateAuth bool              `json:"-" yaml:"-"`
	Vault        *VaultRuntimeAuth `json:"-" yaml:"-"`
	AWS          *AWSRuntimeAuth   `json:"-" yaml:"-"`
}

// VaultRuntimeAuth describes an authenticated Vault medium supplied by an
// upstream package. Token is runtime-only and must not be serialized.
type VaultRuntimeAuth struct {
	Address   string `json:"-" yaml:"-"`
	Namespace string `json:"-" yaml:"-"`
	Token     string `json:"-" yaml:"-"`
}

// AWSRuntimeAuth describes an authenticated AWS medium supplied by an upstream
// package. Credential fields are runtime-only and must not be serialized.
type AWSRuntimeAuth struct {
	Region          string `json:"-" yaml:"-"`
	AccessKeyID     string `json:"-" yaml:"-"`
	SecretAccessKey string `json:"-" yaml:"-"`
	SessionToken    string `json:"-" yaml:"-"`
	RoleARN         string `json:"-" yaml:"-"`
	EndpointURL     string `json:"-" yaml:"-"`
}

// HasStaticCredentials reports whether the runtime AWS session contains enough
// material to create an SDK static credentials provider.
func (a *AWSRuntimeAuth) HasStaticCredentials() bool {
	return a != nil && a.AccessKeyID != "" && a.SecretAccessKey != ""
}

func (a *RuntimeAuth) copy() *RuntimeAuth {
	if a == nil {
		return nil
	}

	out := &RuntimeAuth{DelegateAuth: a.DelegateAuth}
	if a.Vault != nil {
		vault := *a.Vault
		out.Vault = &vault
	}
	if a.AWS != nil {
		aws := *a.AWS
		out.AWS = &aws
	}
	return out
}

func (a *RuntimeAuth) applyToConfig(cfg *Config) {
	if a == nil || a.DelegateAuth || cfg == nil {
		return
	}

	if a.Vault != nil {
		if a.Vault.Address != "" {
			cfg.Vault.Address = a.Vault.Address
		}
		if a.Vault.Namespace != "" {
			cfg.Vault.Namespace = a.Vault.Namespace
		}
		if a.Vault.Token != "" {
			cfg.Vault.Auth.Token = &TokenAuth{Token: a.Vault.Token}
		}
	}

	if a.AWS != nil && a.AWS.Region != "" {
		cfg.AWS.Region = a.AWS.Region
	}
}
