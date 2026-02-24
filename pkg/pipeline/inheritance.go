package pipeline

import (
	"fmt"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

// warnedImports tracks imports that have already been warned about
// to prevent log spam on repeated calls to GetSourcePath
// Using sync.Map for atomic operations without explicit locking
var warnedImports sync.Map

// ValidateTargetInheritance checks for circular dependencies in target inheritance chains
func (c *Config) ValidateTargetInheritance() error {
	for name := range c.Targets {
		visited := make(map[string]bool)
		recursionStack := make(map[string]bool)
		path := []string{} // Track the path for better error messages
		if err := c.detectCycle(name, visited, recursionStack, path); err != nil {
			return err
		}
	}
	return nil
}

// detectCycle performs DFS to detect circular dependencies in target inheritance
// path tracks the current traversal path for detailed error messages
func (c *Config) detectCycle(targetName string, visited, recursionStack map[string]bool, path []string) error {
	visited[targetName] = true
	recursionStack[targetName] = true
	path = append(path, targetName)

	if target, ok := c.Targets[targetName]; ok {
		for _, imp := range target.Imports {
			if _, isTarget := c.Targets[imp]; isTarget {
				// Self-reference is also a cycle
				if imp == targetName {
					return fmt.Errorf("circular dependency detected: %s -> %s (self-reference)", targetName, imp)
				}
				if !visited[imp] {
					if err := c.detectCycle(imp, visited, recursionStack, path); err != nil {
						return err
					}
				} else if recursionStack[imp] {
					// Build full cycle path for clearer error message
					cyclePath := buildCyclePath(path, imp)
					return fmt.Errorf("circular dependency detected in target inheritance: %s", cyclePath)
				}
			}
		}
	}

	recursionStack[targetName] = false
	return nil
}

// buildCyclePath constructs a human-readable cycle path like "A -> B -> C -> A"
func buildCyclePath(path []string, cycleTarget string) string {
	// Find where the cycle starts in the path
	cycleStart := -1
	for i, p := range path {
		if p == cycleTarget {
			cycleStart = i
			break
		}
	}

	if cycleStart == -1 {
		// Cycle target not in path, just show last element -> cycle target
		if len(path) > 0 {
			return path[len(path)-1] + " -> " + cycleTarget
		}
		return cycleTarget
	}

	// Build the cycle path from start to end, plus back to start
	cycleParts := append(path[cycleStart:], cycleTarget)
	return strings.Join(cycleParts, " -> ")
}

// IsInheritedTarget checks if a target inherits from another target
func (c *Config) IsInheritedTarget(targetName string) bool {
	target, ok := c.Targets[targetName]
	if !ok {
		return false
	}
	for _, imp := range target.Imports {
		if _, isTarget := c.Targets[imp]; isTarget {
			return true
		}
	}
	return false
}

// GetSourcePath returns the full path for a source or inherited target
func (c *Config) GetSourcePath(importName string) string {
	if src, ok := c.Sources[importName]; ok {
		if src.Vault != nil {
			return src.Vault.Mount
		}
	}

	if _, ok := c.Targets[importName]; ok {
		if c.MergeStore.Vault != nil {
			return fmt.Sprintf("%s/%s", c.MergeStore.Vault.Mount, importName)
		}
	}

	// Rate-limit warnings to prevent log spam on repeated calls
	// LoadOrStore atomically stores and returns whether the value was loaded (existed)
	if _, loaded := warnedImports.LoadOrStore(importName, true); !loaded {
		// This is the first time we're seeing this import - log a warning
		log.WithField("import", importName).Warn("Unknown import - not found in sources or targets, using import name as path")
	}
	return importName
}

// GetRoleARN returns the role ARN for a target account
func (c *Config) GetRoleARN(accountID string) string {
	for _, target := range c.Targets {
		if target.AccountID == accountID && target.RoleARN != "" {
			return target.RoleARN
		}
	}

	if c.AWS.ControlTower.Enabled {
		roleName := c.AWS.ControlTower.ExecutionRole.Name
		if roleName == "" {
			roleName = "AWSControlTowerExecution"
		}
		path := c.AWS.ControlTower.ExecutionRole.Path
		if path == "" {
			path = "/"
		} else {
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			if !strings.HasSuffix(path, "/") {
				path += "/"
			}
		}
		return fmt.Sprintf("arn:aws:iam::%s:role%s%s", accountID, path, roleName)
	}

	if c.AWS.ExecutionContext.CustomRolePattern != "" {
		return strings.ReplaceAll(c.AWS.ExecutionContext.CustomRolePattern, "{{.AccountID}}", accountID)
	}

	return fmt.Sprintf("arn:aws:iam::%s:role/AWSControlTowerExecution", accountID)
}
