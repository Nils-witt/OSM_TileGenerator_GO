package main

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
)

// cliCase is one invocation of run and what it should produce.
type cliCase struct {
	name       string
	args       []string
	wantErr    error // matched with errors.Is; nil means success
	wantAnyErr bool  // any non-usage error, e.g. a missing input directory
	wantStdout string
	wantStderr string
	// rejectStderr, when set, must not appear on stderr.
	rejectStderr string
}

func runCLICases(t *testing.T, tests []cliCase) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer

			err := run(tt.args, &stdout, &stderr)

			switch {
			case tt.wantAnyErr:
				if err == nil || errors.Is(err, errUsage) {
					t.Errorf("run(%q) error = %v, want a non-usage error", tt.args, err)
				}
			case tt.wantErr == nil:
				if err != nil {
					t.Errorf("run(%q) error = %v, want nil", tt.args, err)
				}
			default:
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("run(%q) error = %v, want %v", tt.args, err, tt.wantErr)
				}
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
			if tt.rejectStderr != "" && strings.Contains(stderr.String(), tt.rejectStderr) {
				t.Errorf("stderr = %q, want it not to contain %q", stderr.String(), tt.rejectStderr)
			}
		})
	}
}

func TestRun(t *testing.T) {
	t.Parallel()

	runCLICases(t, []cliCase{
		{
			name:       "help command prints usage",
			args:       []string{"help"},
			wantStdout: "Commands:",
		},
		{
			name:       "help flag prints usage",
			args:       []string{"-h"},
			wantStdout: "Commands:",
		},
		{
			name:       "version command prints version",
			args:       []string{"version"},
			wantStdout: "osm-tile-generator",
		},
		{
			name:       "version command prints repository url",
			args:       []string{"version"},
			wantStdout: "\n" + repoURL + "\n",
		},
		{
			name:       "unknown command is a usage error",
			args:       []string{"render"},
			wantErr:    errUsage,
			wantStderr: `unknown command "render"`,
		},
		{
			name:       "subcommand help returns ErrHelp",
			args:       []string{"generate", "-h"},
			wantErr:    flag.ErrHelp,
			wantStderr: "osm-tile-generator generate [flags]",
		},
	})
}
