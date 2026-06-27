package azure

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/jbcom/secrets-sync/pkg/driver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// fakeAPI is an in-memory secretsAPI.
type fakeAPI struct {
	store map[string]string
}

func newFakeAPI() *fakeAPI { return &fakeAPI{store: map[string]string{}} }

func (f *fakeAPI) SetSecret(_ context.Context, name string, params azsecrets.SetSecretParameters, _ *azsecrets.SetSecretOptions) (azsecrets.SetSecretResponse, error) {
	if params.Value == nil {
		return azsecrets.SetSecretResponse{}, fmt.Errorf("nil value")
	}
	f.store[name] = *params.Value
	return azsecrets.SetSecretResponse{}, nil
}

func (f *fakeAPI) GetSecret(_ context.Context, name, _ string, _ *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
	v, ok := f.store[name]
	if !ok {
		return azsecrets.GetSecretResponse{}, fmt.Errorf("not found: %s", name)
	}
	val := v
	return azsecrets.GetSecretResponse{Secret: azsecrets.Secret{Value: &val}}, nil
}

func (f *fakeAPI) DeleteSecret(_ context.Context, name string, _ *azsecrets.DeleteSecretOptions) (azsecrets.DeleteSecretResponse, error) {
	delete(f.store, name)
	return azsecrets.DeleteSecretResponse{}, nil
}

func (f *fakeAPI) ListSecretNames(context.Context) ([]string, error) {
	out := make([]string, 0, len(f.store))
	for k := range f.store {
		out = append(out, k)
	}
	return out, nil
}

func newTestClient() (*Client, *fakeAPI) {
	fake := newFakeAPI()
	c := &Client{VaultURL: "https://v.vault.azure.net/", api: fake}
	_ = c.Init(context.Background())
	return c, fake
}

func TestNewRequiresVaultURL(t *testing.T) {
	if _, err := New(driver.BackendSpec{Driver: driver.DriverNameAzure}); err == nil {
		t.Fatal("expected error when vault_url missing")
	}
	c, err := New(driver.BackendSpec{Driver: driver.DriverNameAzure, Path: "https://v.vault.azure.net/"})
	if err != nil || c.VaultURL == "" {
		t.Fatalf("New: c=%+v err=%v", c, err)
	}
}

func TestWriteGetRoundTrip(t *testing.T) {
	ctx := context.Background()
	c, fake := newTestClient()
	if _, err := c.WriteSecret(ctx, metav1.ObjectMeta{}, "app/db", []byte(`{"u":"p"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if fake.store["app-db"] != `{"u":"p"}` {
		t.Fatalf("name sanitization or store wrong: %v", fake.store)
	}
	got, err := c.GetSecret(ctx, "app/db")
	if err != nil || string(got) != `{"u":"p"}` {
		t.Fatalf("get: got=%s err=%v", got, err)
	}
}

func TestListAndDelete(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestClient()
	_, _ = c.WriteSecret(ctx, metav1.ObjectMeta{}, "a", []byte(`{"x":"1"}`))
	_, _ = c.WriteSecret(ctx, metav1.ObjectMeta{}, "b", []byte(`{"y":"2"}`))
	names, err := c.ListSecrets(ctx, "")
	if err != nil || len(names) != 2 {
		t.Fatalf("list: names=%v err=%v", names, err)
	}
	if err := c.DeleteSecret(ctx, "a"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	names, _ = c.ListSecrets(ctx, "")
	if len(names) != 1 {
		t.Fatalf("expected 1 after delete, got %v", names)
	}
}

func TestSecretNameSanitization(t *testing.T) {
	cases := map[string]string{
		"app/db":     "app-db",
		"App_Config": "App-Config",
		"a.b.c":      "a-b-c",
		"/x/":        "x",
		"":           "secret",
		"---":        "secret",
	}
	for in, want := range cases {
		if got := secretName(in); got != want {
			t.Fatalf("secretName(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestGetMissingErrors(t *testing.T) {
	c, _ := newTestClient()
	if _, err := c.GetSecret(context.Background(), "nope"); err == nil {
		t.Fatal("expected error for missing secret")
	}
}
