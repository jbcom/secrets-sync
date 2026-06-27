package secrets_sync_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var (
	actionRefPattern              = regexp.MustCompile(`^\s*(?:-\s*)?uses:\s*([^#\s]+)(?:\s+#\s*(\S+))?`)
	actionVersionCommentPattern   = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
	pinnedSHAPattern              = regexp.MustCompile(`^[0-9a-f]{40}$`)
	publishingChecklistPinPattern = regexp.MustCompile("^\\|\\s*`([^`]+)`\\s*\\|\\s*`([^`]+)`\\s*\\|\\s*`([0-9a-f]{40})`\\s*\\|$")
)

type workflowActionPin struct {
	Version string
	SHA     string
}

func TestWorkflowActionsArePinnedToExactSHAs(t *testing.T) {
	pins := workflowActionPins(t)
	if len(pins) == 0 {
		t.Fatalf("expected workflow action pins")
	}
}

func TestPublishingChecklistMatchesWorkflowActionPins(t *testing.T) {
	workflowPins := workflowActionPins(t)
	checklistPins := publishingChecklistPins(t)

	if len(checklistPins) == 0 {
		t.Fatalf("docs/PUBLISHING_CHECKLIST.md must list current workflow action pins")
	}
	if len(workflowPins) != len(checklistPins) {
		t.Fatalf("publishing checklist pin count mismatch: workflow=%d checklist=%d", len(workflowPins), len(checklistPins))
	}
	for action, workflowPin := range workflowPins {
		checklistPin, ok := checklistPins[action]
		if !ok {
			t.Fatalf("docs/PUBLISHING_CHECKLIST.md missing workflow action %s", action)
		}
		if checklistPin != workflowPin {
			t.Fatalf("docs/PUBLISHING_CHECKLIST.md pin for %s = %+v, want %+v", action, checklistPin, workflowPin)
		}
	}
	for action := range checklistPins {
		if _, ok := workflowPins[action]; !ok {
			t.Fatalf("docs/PUBLISHING_CHECKLIST.md lists non-workflow action %s", action)
		}
	}
}

func TestMaintainedDocsPinThirdPartyActions(t *testing.T) {
	paths := markdownAndExampleWorkflowFiles(t)
	var offenders []string

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		for index, line := range strings.Split(string(content), "\n") {
			matches := actionRefPattern.FindStringSubmatch(line)
			if matches == nil {
				continue
			}

			uses := strings.TrimSpace(matches[1])
			if shouldSkipDocumentedActionPin(uses) {
				continue
			}

			_, ref, found := strings.Cut(uses, "@")
			version := ""
			if len(matches) > 2 {
				version = matches[2]
			}
			if !found || !pinnedSHAPattern.MatchString(ref) {
				offenders = append(offenders, path+":"+itoa(index+1)+": "+uses)
				continue
			}
			if !actionVersionCommentPattern.MatchString(version) {
				offenders = append(offenders, path+":"+itoa(index+1)+": missing stable version comment for "+uses)
			}
		}
	}

	if len(offenders) > 0 {
		t.Fatalf("maintained docs/examples must pin third-party actions to exact commit SHAs:\n%s", strings.Join(offenders, "\n"))
	}
}

func workflowActionPins(t *testing.T) map[string]workflowActionPin {
	t.Helper()

	workflowRoot := filepath.Join(".github", "workflows")
	entries, err := os.ReadDir(workflowRoot)
	if err != nil {
		t.Fatalf("read workflow directory: %v", err)
	}

	pins := map[string]workflowActionPin{}
	var offenders []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}

		path := filepath.Join(workflowRoot, name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read workflow %s: %v", path, err)
		}

		for index, line := range strings.Split(string(content), "\n") {
			matches := actionRefPattern.FindStringSubmatch(line)
			if matches == nil {
				continue
			}

			uses := strings.TrimSpace(matches[1])
			if strings.HasPrefix(uses, "./") || strings.HasPrefix(uses, "docker://") {
				continue
			}

			action, ref, found := strings.Cut(uses, "@")
			version := ""
			if len(matches) > 2 {
				version = matches[2]
			}
			if !found || !pinnedSHAPattern.MatchString(ref) {
				offenders = append(offenders, path+":"+itoa(index+1)+": "+uses)
				continue
			}
			if !actionVersionCommentPattern.MatchString(version) {
				offenders = append(offenders, path+":"+itoa(index+1)+": missing stable version comment for "+uses)
				continue
			}

			pin := workflowActionPin{Version: version, SHA: ref}
			if existing, ok := pins[action]; ok && existing != pin {
				offenders = append(offenders, path+":"+itoa(index+1)+": conflicting pin for "+action)
				continue
			}
			pins[action] = pin
		}
	}

	if len(offenders) > 0 {
		t.Fatalf("workflow actions must be pinned to exact commit SHAs:\n%s", strings.Join(offenders, "\n"))
	}
	return pins
}

func markdownAndExampleWorkflowFiles(t *testing.T) []string {
	t.Helper()

	roots := []string{"README.md", "docs", "examples"}
	var paths []string
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			t.Fatalf("stat %s: %v", root, err)
		}
		if !info.IsDir() {
			paths = append(paths, root)
			continue
		}
		err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			switch filepath.Ext(path) {
			case ".md", ".yml", ".yaml":
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	return paths
}

func shouldSkipDocumentedActionPin(uses string) bool {
	if strings.HasPrefix(uses, "./") || strings.HasPrefix(uses, "docker://") {
		return true
	}
	if strings.HasPrefix(uses, "jbcom/secrets-sync@") {
		return true
	}
	return false
}

func publishingChecklistPins(t *testing.T) map[string]workflowActionPin {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("docs", "PUBLISHING_CHECKLIST.md"))
	if err != nil {
		t.Fatalf("read publishing checklist: %v", err)
	}

	pins := map[string]workflowActionPin{}
	for _, line := range strings.Split(string(content), "\n") {
		matches := publishingChecklistPinPattern.FindStringSubmatch(strings.TrimSpace(line))
		if matches == nil {
			continue
		}
		pins[matches[1]] = workflowActionPin{Version: matches[2], SHA: matches[3]}
	}
	return pins
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
