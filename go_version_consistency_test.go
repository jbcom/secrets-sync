package secrets_sync_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The primary Go toolchain version is declared in exactly one place the build
// actually consumes -- the Dockerfile builder stage -- and every other site
// must agree with it. Workflows, the Justfile and the docs each repeat the
// version as a literal because their formats offer no include mechanism, so
// without a test a bump silently lands in some files and not others. That
// exact drift is what produced an earlier Go runtime-fix branch, where
// CI ran one patch release while the Justfile and Dockerfile ran another.
const goVersionSourceOfTruth = "Dockerfile"

var (
	dockerfileGoVersionPattern = regexp.MustCompile(`(?m)^FROM golang:(\d+\.\d+\.\d+)-`)
	setupGoVersionPattern      = regexp.MustCompile(`go-version:\s*"(\d+\.\d+\.\d+)"`)
	goToolchainPattern         = regexp.MustCompile(`GOTOOLCHAIN="?\$\{GO_TOOLCHAIN:-go(\d+\.\d+\.\d+)\}"?`)
	inlineToolchainPattern     = regexp.MustCompile(`GOTOOLCHAIN=go(\d+\.\d+\.\d+)\b`)
	ciMatrixPattern            = regexp.MustCompile(`go-version:\s*\[([^\]]+)\]`)
	quotedVersionPattern       = regexp.MustCompile(`\d+\.\d+\.\d+`)
)

// primaryGoVersion reads the single declared version from the Dockerfile.
func primaryGoVersion(t *testing.T) string {
	t.Helper()

	content, err := os.ReadFile(goVersionSourceOfTruth)
	if err != nil {
		t.Fatalf("read %s: %v", goVersionSourceOfTruth, err)
	}

	match := dockerfileGoVersionPattern.FindStringSubmatch(string(content))
	if match == nil {
		t.Fatalf("%s does not declare a golang builder image version", goVersionSourceOfTruth)
	}

	return match[1]
}

func TestGoToolchainVersionIsConsistentAcrossBuildSurfaces(t *testing.T) {
	want := primaryGoVersion(t)

	type site struct {
		path     string
		patterns []*regexp.Regexp
	}

	sites := []site{
		{filepath.Join(".github", "workflows", "ci.yml"), []*regexp.Regexp{setupGoVersionPattern}},
		{filepath.Join(".github", "workflows", "cd.yml"), []*regexp.Regexp{setupGoVersionPattern, inlineToolchainPattern}},
		{"Justfile", []*regexp.Regexp{goToolchainPattern}},
	}

	for _, s := range sites {
		content, err := os.ReadFile(s.path)
		if err != nil {
			t.Fatalf("read %s: %v", s.path, err)
		}

		var found int
		for _, pattern := range s.patterns {
			for _, match := range pattern.FindAllStringSubmatch(string(content), -1) {
				found++
				if match[1] != want {
					t.Errorf("%s pins Go %s but %s declares %s", s.path, match[1], goVersionSourceOfTruth, want)
				}
			}
		}

		if found == 0 {
			t.Errorf("%s declares no Go version; the consistency guard would silently pass", s.path)
		}
	}
}

// The CI matrix intentionally spans multiple releases, but the newest entry is
// the one that must track the Dockerfile -- that entry gates govulncheck and
// the release build.
func TestCIMatrixNewestGoVersionMatchesPrimary(t *testing.T) {
	want := primaryGoVersion(t)

	versions := ciMatrixGoVersions(t)
	newest := versions[len(versions)-1]

	if newest != want {
		t.Errorf(".github/workflows/ci.yml matrix newest entry is Go %s but %s declares %s",
			newest, goVersionSourceOfTruth, want)
	}
}

// Documentation quotes the toolchain version for people installing from
// source; a stale number sends them to a release the build no longer uses.
func TestDocsQuoteCurrentGoVersion(t *testing.T) {
	want := primaryGoVersion(t)

	// The docs legitimately name every version the CI matrix supports, not just
	// the primary one, so the matrix defines what is allowed. Deriving the set
	// this way also keeps the check alive across release lines: hard-coding the
	// current minor would make the guard blind the moment the Dockerfile moves
	// to a new one, which is precisely when stale docs appear.
	allowed := map[string]bool{}
	for _, v := range ciMatrixGoVersions(t) {
		allowed[v] = true
	}
	allowed[want] = true

	// docs/dist is generated Sourcey output and is not tracked as source.
	docs := []string{
		filepath.Join("docs", "PIPELINE.md"),
		filepath.Join("docs", "getting-started", "installation.md"),
		"CLAUDE.md",
	}

	// Only match versions written as a Go toolchain reference, so unrelated
	// semver in the docs (chart versions, action tags) is not swept up.
	quoted := regexp.MustCompile(`(?i)\b(?:go|golang:)\s*v?(\d+\.\d+\.\d+)\b`)

	for _, path := range docs {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		for _, match := range quoted.FindAllStringSubmatch(string(content), -1) {
			if !allowed[match[1]] {
				t.Errorf("%s references Go %s, which is neither the primary version (%s) nor in the CI matrix",
					path, match[1], want)
			}
		}
	}
}

// ciMatrixGoVersions returns every version listed in the CI go-version matrix.
func ciMatrixGoVersions(t *testing.T) []string {
	t.Helper()

	path := filepath.Join(".github", "workflows", "ci.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	match := ciMatrixPattern.FindStringSubmatch(string(content))
	if match == nil {
		t.Fatalf("%s does not declare a go-version matrix", path)
	}

	versions := quotedVersionPattern.FindAllString(match[1], -1)
	if len(versions) == 0 {
		t.Fatalf("%s go-version matrix declares no versions", path)
	}

	return versions
}

// A bump must not leave the module's own floor above the toolchain it runs on.
func TestGoModDirectiveNotNewerThanToolchain(t *testing.T) {
	want := primaryGoVersion(t)

	content, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}

	match := regexp.MustCompile(`(?m)^go\s+(\d+\.\d+)`).FindStringSubmatch(string(content))
	if match == nil {
		t.Fatalf("go.mod does not declare a go directive")
	}

	declared := match[1]
	toolchainMinor := strings.Join(strings.Split(want, ".")[:2], ".")

	if compareMinor(declared, toolchainMinor) > 0 {
		t.Errorf("go.mod requires Go %s but the toolchain is %s", declared, want)
	}
}

// compareMinor orders two "major.minor" strings numerically so that 1.9 sorts
// below 1.10, which a lexical comparison would get backwards.
func compareMinor(a, b string) int {
	var aMajor, aMinor, bMajor, bMinor int
	fmt.Sscanf(a, "%d.%d", &aMajor, &aMinor)
	fmt.Sscanf(b, "%d.%d", &bMajor, &bMinor)

	switch {
	case aMajor != bMajor:
		return aMajor - bMajor
	default:
		return aMinor - bMinor
	}
}
