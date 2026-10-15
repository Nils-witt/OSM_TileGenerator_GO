package main

import (
	"flag"
	"io"

	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/pipeline"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/version"
	"github.com/Nils-witt/OSM_TileGenerator_GO/internal/workspace"
)

// generateFlags holds the flags shared by the generate command and the
// deprecated no-subcommand invocation.
type generateFlags struct {
	configDir string
	minZoom   int
	maxZoom   int
}

func (f *generateFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&f.configDir, "config-dir", "config", "Directory containing config.json and image.png")
	fs.IntVar(&f.minZoom, "min-zoom", 14, "Minimum zoom level")
	fs.IntVar(&f.maxZoom, "max-zoom", 18, "Maximum zoom level")
}

// validate rejects zoom ranges that would silently produce nothing.
func (f *generateFlags) validate(fs *flag.FlagSet) error {
	if f.minZoom < 0 {
		return usageErrorf(fs, "-min-zoom must not be negative, got %d", f.minZoom)
	}
	if f.minZoom > f.maxZoom {
		return usageErrorf(fs, "-min-zoom (%d) must not exceed -max-zoom (%d)", f.minZoom, f.maxZoom)
	}
	return nil
}

func runGenerate(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("generate", "Warp <config-dir>/image.png into a tile pyramid and zip it.", stderr)
	var flags generateFlags
	flags.register(fs)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := flags.validate(fs); err != nil {
		return err
	}

	return generate(flags, stdout)
}

func generate(flags generateFlags, stdout io.Writer) error {
	writef(stdout, "%s\n", version.String())
	return pipeline.Run(pipeline.Options{
		Dir:     workspace.Dir(flags.configDir),
		MinZoom: flags.minZoom,
		MaxZoom: flags.maxZoom,
	})
}
