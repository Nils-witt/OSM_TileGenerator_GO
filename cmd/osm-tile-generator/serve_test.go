package main

import (
	"path/filepath"
	"testing"
)

func TestRunServe(t *testing.T) {
	t.Parallel()

	missingDir := filepath.Join(t.TempDir(), "missing")
	runCLICases(t, []cliCase{
		{
			name:       "port zero is a usage error",
			args:       []string{"serve", "-port", "0"},
			wantErr:    errUsage,
			wantStderr: "-port must be between 1 and 65535, got 0",
		},
		{
			name:       "port above range is a usage error",
			args:       []string{"serve", "-port", "70000"},
			wantErr:    errUsage,
			wantStderr: "-port must be between 1 and 65535, got 70000",
		},
		{
			name:       "missing tiles directory is a runtime error",
			args:       []string{"serve", "-config-dir", missingDir},
			wantAnyErr: true,
			wantStderr: "level=INFO msg=starting command=serve",
		},
	})
}
