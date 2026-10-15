package main

import (
	"fmt"
	"io"
)

// runLegacy keeps the pre-subcommand invocation working:
//
//	osm-tile-generator [-config-dir dir] [-min-zoom z] [-max-zoom z]
//	osm-tile-generator [-config-dir dir] -serve port
//
// It prints a deprecation notice and forwards to generate or serve.
//
// Deprecated: remove once users have moved to the generate and serve commands.
func runLegacy(args []string, stderr io.Writer) error {
	fs := newFlagSet("", "Deprecated: use the generate or serve command instead.", stderr)
	var flags generateFlags
	flags.register(fs)
	servePort := fs.Int("serve", 0, "Deprecated: use the serve command")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	if *servePort != 0 {
		if err := validatePort(fs, "-serve", *servePort); err != nil {
			return err
		}
		writef(stderr, "%s: -serve is deprecated; use %q instead\n",
			programName, fmt.Sprintf("%s serve -config-dir %s -port %d", programName, flags.configDir, *servePort))
		return serve(flags.configDir, *servePort, flags.log.newLogger(stderr))
	}

	writef(stderr, "%s: running without a command is deprecated; use %q instead\n",
		programName, programName+" generate")
	if err := flags.validate(fs); err != nil {
		return err
	}
	return generate(flags, flags.log.newLogger(stderr))
}
