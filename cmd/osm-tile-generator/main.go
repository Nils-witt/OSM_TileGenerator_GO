// Command osm-tile-generator georeferences a source image onto the OSM tile
// grid and serves the resulting tiles.
//
// Usage:
//
//	osm-tile-generator generate [-config-dir dir] [-min-zoom z] [-max-zoom z]
//	osm-tile-generator serve [-config-dir dir] [-port port]
//	osm-tile-generator version
package main

import (
	"errors"
	"flag"
	"fmt"
	_ "image/png"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/version"
)

const (
	programName = "osm-tile-generator"
	repoURL     = "https://github.com/Nils-witt/OSM_TileGenerator_GO"
)

// errUsage is returned for command-line mistakes. The details have already
// been written to stderr where the mistake was detected; main only maps it to
// exit status 2.
var errUsage = errors.New("invalid usage")

const usageText = `Usage:
  osm-tile-generator <command> [flags]

Commands:
  generate   Warp <config-dir>/image.png into a tile pyramid and zip it
  serve      Serve <config-dir>/tiles over HTTP
  version    Print version information and the project URL

Run "osm-tile-generator <command> -h" for a command's flags.
`

func main() {
	err := run(os.Args[1:], os.Stdout, os.Stderr)
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return
	case errors.Is(err, errUsage):
		os.Exit(2)
	default:
		writef(os.Stderr, "%s: %v\n", programName, err)
		os.Exit(1)
	}
}

// run dispatches args (without the program name) to a subcommand.
func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || (strings.HasPrefix(args[0], "-") && !isHelpFlag(args[0])) {
		return runLegacy(args, stderr)
	}

	switch cmd, rest := args[0], args[1:]; cmd {
	case "generate":
		return runGenerate(rest, stderr)
	case "serve":
		return runServe(rest, stderr)
	case "version":
		_, err := fmt.Fprintf(stdout, "%s\n%s\n", version.String(), repoURL)
		return err
	case "help", "-h", "-help", "--help":
		_, err := fmt.Fprint(stdout, usageText)
		return err
	default:
		writef(stderr, "%s: unknown command %q\n\n%s", programName, cmd, usageText)
		return errUsage
	}
}

func isHelpFlag(arg string) bool {
	switch arg {
	case "-h", "-help", "--help":
		return true
	}
	return false
}

// writef writes diagnostic text to w. Write errors are deliberately ignored:
// when stdout or stderr is gone there is nowhere left to report them.
func writef(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}

// newFlagSet returns a flag set for subcommand name (empty for the bare
// program) that reports parse errors instead of exiting, so run stays testable.
func newFlagSet(name, synopsis string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(strings.TrimSpace(programName+" "+name), flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		writef(stderr, "Usage:\n  %s [flags]\n\n%s\n\nFlags:\n", fs.Name(), synopsis)
		fs.PrintDefaults()
	}
	return fs
}

// parseFlags parses args into fs and rejects positional arguments. Errors are
// already reported on fs's output; flag.ErrHelp is passed through so -h exits 0.
func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return errUsage
	}
	if fs.NArg() > 0 {
		return usageErrorf(fs, "unexpected argument %q", fs.Arg(0))
	}
	return nil
}

// logFlags holds the logging flags every long-running command accepts.
type logFlags struct {
	level slog.Level
}

func (f *logFlags) register(fs *flag.FlagSet) {
	fs.TextVar(&f.level, "log-level", slog.LevelInfo, "Minimum log level: debug, info, warn or error")
}

// newLogger returns a text logger writing to w at the configured level. Logs
// always go to stderr so stdout stays free for program output.
func (f *logFlags) newLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: f.level}))
}

// usageErrorf reports a usage mistake together with fs's usage text.
func usageErrorf(fs *flag.FlagSet, format string, a ...any) error {
	writef(fs.Output(), "%s: %s\n", fs.Name(), fmt.Sprintf(format, a...))
	fs.Usage()
	return errUsage
}
