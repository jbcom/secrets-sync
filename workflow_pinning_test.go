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
	actionRefPattern = regexp.MustCompile(`^\s*(?:-\s*)?uses:\s*([^#\s]+)`)
	pinnedSHAPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

func TestWorkflowActionsArePinnedToExactSHAs(t *testing.T) {
	workflowRoot := filepath.Join(".github", "workflows")
	entries, err := os.ReadDir(workflowRoot)
	if err != nil {
		t.Fatalf("read workflow directory: %v", err)
	}

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

			parts := strings.Split(uses, "@")
			ref := ""
			if len(parts) == 2 {
				ref = parts[1]
			}
			if !pinnedSHAPattern.MatchString(ref) {
				offenders = append(offenders, path+":"+itoa(index+1)+": "+uses)
			}
		}
	}

	if len(offenders) > 0 {
		t.Fatalf("workflow actions must be pinned to exact commit SHAs:\n%s", strings.Join(offenders, "\n"))
	}
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
