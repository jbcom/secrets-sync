package cmd

import (
	"strings"
	"testing"
)

func TestCommandHelpDoesNotAdvertiseVSSAlias(t *testing.T) {
	commands := []*cobraCommandText{
		{name: "context", long: contextCmd.Long},
		{name: "graph", long: graphCmd.Long},
		{name: "migrate", long: migrateCmd.Long},
		{name: "validate", long: validateCmd.Long},
	}

	for _, command := range commands {
		t.Run(command.name, func(t *testing.T) {
			if strings.Contains(command.long, "vss ") || strings.Contains(command.long, "./vss") {
				t.Fatalf("%s help should advertise secretsync, not vss:\n%s", command.name, command.long)
			}
			if !strings.Contains(command.long, "secretsync ") {
				t.Fatalf("%s help should include secretsync examples:\n%s", command.name, command.long)
			}
		})
	}
}

type cobraCommandText struct {
	name string
	long string
}
