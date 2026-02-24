package pipeline

import (
	"testing"
)

func TestBundleID_Deterministic(t *testing.T) {
	sources := []string{"vault/source1", "vault/source2", "vault/source3"}

	// Same sources should always produce same ID
	id1 := BundleID(sources)
	id2 := BundleID(sources)

	if id1 != id2 {
		t.Errorf("BundleID should be deterministic: got %s and %s", id1, id2)
	}
}

func TestBundleID_OrderMatters(t *testing.T) {
	sources1 := []string{"vault/source1", "vault/source2"}
	sources2 := []string{"vault/source2", "vault/source1"}

	id1 := BundleID(sources1)
	id2 := BundleID(sources2)

	if id1 == id2 {
		t.Errorf("BundleID should differ for different order: both got %s", id1)
	}
}

func TestBundleID_DifferentSources(t *testing.T) {
	sources1 := []string{"vault/source1"}
	sources2 := []string{"vault/source2"}

	id1 := BundleID(sources1)
	id2 := BundleID(sources2)

	if id1 == id2 {
		t.Errorf("BundleID should differ for different sources: both got %s", id1)
	}
}

func TestBundlePath(t *testing.T) {
	sources := []string{"vault/source1", "vault/source2"}
	mount := "merged-secrets"

	path := BundlePath(mount, sources)
	expected := "merged-secrets/bundles/" + BundleID(sources)

	if path != expected {
		t.Errorf("BundlePath: got %s, want %s", path, expected)
	}
}

func TestTargetBundlePath(t *testing.T) {
	sources := []string{"vault/source1", "vault/source2"}
	mount := "merged-secrets"
	target := "Production"

	path := TargetBundlePath(mount, target, sources)
	expected := "merged-secrets/targets/Production/" + BundleID(sources)

	if path != expected {
		t.Errorf("TargetBundlePath: got %s, want %s", path, expected)
	}
}

func TestPipelineRequest_GetBundleID(t *testing.T) {
	req := &PipelineRequest{
		Sources: []string{"vault/source1", "vault/source2"},
		Targets: []string{"target1", "target2"},
	}

	id1 := req.GetBundleID()
	id2 := BundleID(req.Sources)

	if id1 != id2 {
		t.Errorf("PipelineRequest.GetBundleID should match BundleID: got %s, want %s", id1, id2)
	}
}
