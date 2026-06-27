package secrets_sync_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDocsDoNotAdvertiseOldCurrentVersion(t *testing.T) {
	forbiddenByPath := map[string][]string{
		"docs/ROADMAP.md": {
			"Current Status: v1.2.0",
			"Future Considerations (v2.0+)",
			"### v1.3.0",
			"### v1.4.0",
			"### v1.5.0",
			"coming in v1.3.0",
		},
		"docs/FAQ.md": {
			"SecretSync v1.2.0 is production-ready",
		},
	}

	for path, forbidden := range forbiddenByPath {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		text := string(content)
		for _, phrase := range forbidden {
			if strings.Contains(text, phrase) {
				t.Fatalf("%s should not advertise old current-version phrase %q", path, phrase)
			}
		}
	}
}

func TestPublicDocsDoNotAdvertiseOldFeatureReleaseLabels(t *testing.T) {
	forbidden := []string{"v1.1.0", "v1.2.0"}
	paths := []string{"README.md"}

	if err := filepath.WalkDir("docs", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		t.Fatalf("walk docs: %v", err)
	}

	var offenders []string
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		text := string(content)
		for _, phrase := range forbidden {
			if strings.Contains(text, phrase) {
				offenders = append(offenders, path+": "+phrase)
			}
		}
	}

	if len(offenders) > 0 {
		t.Fatalf("public docs should not advertise old feature-release labels:\n%s", strings.Join(offenders, "\n"))
	}
}

func TestDocsImageTagMatchesDefaultImageConstant(t *testing.T) {
	source, err := os.ReadFile("pkg/kubernetes/controller.go")
	if err != nil {
		t.Fatalf("read pkg/kubernetes/controller.go: %v", err)
	}
	re := regexp.MustCompile(`DefaultImage\s+=\s+"ghcr.io/jbcom/secrets-sync:(v[0-9]+\.[0-9]+\.[0-9]+)"`)
	m := re.FindStringSubmatch(string(source))
	if m == nil {
		t.Fatal("could not extract DefaultImage version from pkg/kubernetes/controller.go")
	}
	currentTag := m[1]
	imageTagRx := regexp.MustCompile(`ghcr\.io/jbcom/secrets-sync:(v[0-9]+\.[0-9]+\.[0-9]+)`)

	var offenders []string
	roots := []string{"docs", "README.md", "deploy"}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			ext := filepath.Ext(path)
			if ext != ".md" && ext != ".yaml" && ext != ".yml" {
				return nil
			}
			if strings.Contains(path, "docs/_build/") {
				return nil
			}
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			for _, match := range imageTagRx.FindAllStringSubmatch(string(content), -1) {
				tag := match[1]
				if tag != currentTag {
					offenders = append(offenders, path+" references stale tag "+tag+" (current: "+currentTag+")")
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("docs/deploy should not reference stale image tags (current: %s):\n%s", currentTag, strings.Join(offenders, "\n"))
	}
}
