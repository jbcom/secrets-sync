package driver

import (
	"errors"
	"testing"
)

func TestRegistryTargetAlsoRegistersSource(t *testing.T) {
	r := NewRegistry()
	const d DriverName = "fake-target"
	r.RegisterTarget(d, func(spec BackendSpec) (TargetBackend, error) {
		b := newFakeBackend(d)
		b.path = spec.Path
		return b, nil
	})

	if !r.SupportsTarget(d) {
		t.Fatal("expected target support")
	}
	if !r.SupportsSource(d) {
		t.Fatal("registering a target must also expose a source")
	}

	src, err := r.NewSource(BackendSpec{Driver: d, Path: "p"})
	if err != nil {
		t.Fatalf("new source: %v", err)
	}
	if src.GetPath() != "p" {
		t.Fatalf("spec.Path not threaded: %q", src.GetPath())
	}

	tgt, err := r.NewTarget(BackendSpec{Driver: d})
	if err != nil {
		t.Fatalf("new target: %v", err)
	}
	if tgt.Driver() != d {
		t.Fatalf("wrong driver: %q", tgt.Driver())
	}
}

func TestRegistryDedicatedSourceWins(t *testing.T) {
	r := NewRegistry()
	const d DriverName = "src-only"
	r.RegisterSource(d, func(BackendSpec) (SourceBackend, error) {
		return newFakeBackend(d), nil
	})
	if r.SupportsTarget(d) {
		t.Fatal("source-only driver must not report target support")
	}
	if _, err := r.NewTarget(BackendSpec{Driver: d}); err == nil {
		t.Fatal("expected error constructing target for source-only driver")
	}
}

func TestRegistryUnknownDriver(t *testing.T) {
	r := NewRegistry()
	if _, err := r.NewSource(BackendSpec{Driver: "nope"}); err == nil {
		t.Fatal("expected error for unknown source driver")
	}
	if _, err := r.NewTarget(BackendSpec{Driver: "nope"}); err == nil {
		t.Fatal("expected error for unknown target driver")
	}
}

func TestRegistryDuplicatePanics(t *testing.T) {
	r := NewRegistry()
	const d DriverName = "dup"
	r.RegisterSource(d, func(BackendSpec) (SourceBackend, error) { return newFakeBackend(d), nil })
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate source registration")
		}
	}()
	r.RegisterSource(d, func(BackendSpec) (SourceBackend, error) { return newFakeBackend(d), nil })
}

func TestRegistryFactoryError(t *testing.T) {
	r := NewRegistry()
	const d DriverName = "boom"
	want := errors.New("construction failed")
	r.RegisterTarget(d, func(BackendSpec) (TargetBackend, error) { return nil, want })
	if _, err := r.NewTarget(BackendSpec{Driver: d}); !errors.Is(err, want) {
		t.Fatalf("expected wrapped factory error, got %v", err)
	}
}

func TestRegistryDriversSorted(t *testing.T) {
	r := NewRegistry()
	r.RegisterSource("zeta", func(BackendSpec) (SourceBackend, error) { return newFakeBackend("zeta"), nil })
	r.RegisterTarget("alpha", func(BackendSpec) (TargetBackend, error) { return newFakeBackend("alpha"), nil })
	got := r.Drivers()
	if len(got) != 2 || got[0] != "alpha" || got[1] != "zeta" {
		t.Fatalf("expected sorted [alpha zeta], got %v", got)
	}
}

func TestDriverIsSupportedBuiltins(t *testing.T) {
	for _, d := range []DriverName{DriverNameAws, DriverNameVault, DriverNameAzure, DriverNameGCP, DriverNameKubernetes, DriverNameHTTP} {
		if !DriverIsSupported(d) {
			t.Fatalf("built-in driver %q should be supported", d)
		}
	}
	if DriverIsSupported("totally-unknown") {
		t.Fatal("unknown driver must not be supported")
	}
}
