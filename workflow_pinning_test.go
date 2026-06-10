package secretsync_test

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
