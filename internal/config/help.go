package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

// PrintHelp prints the description followed by pflag's own option table.
//
// The table comes from PrintDefaults, which already knows the shorthands, the
// value placeholders (taken from a back-quoted word in the usage string) and
// every non-zero default — nothing is kept in sync by hand.
func PrintHelp() {
	opts := defaultOptions()
	fs := newFlagSet(&opts, &meta{})
	fs.SetOutput(os.Stdout)

	fmt.Print(`gofs is a distinctive utility file server.

Usage: gofs [OPTIONS] [serve-path]

Arguments:
  [serve-path]  Specific path to serve (default ".")

Options:
`)
	fs.PrintDefaults()
	fmt.Print("\nConfiguration comes from a TOML file (--config) and/or the command line;\n" +
		"an explicit flag wins over the file.\n")
}

// PrintCompletions prints a shell completion script. Only bash is implemented;
// the script itself is generated from the FlagSet for the same reason as help.
func PrintCompletions(shell string) {
	if shell != "bash" {
		fmt.Printf("# completion for %s is not implemented\n", shell)
		return
	}

	// Every flag name, in registration order, plus its shorthand when it has one.
	var names []string
	opts := defaultOptions()
	fs := newFlagSet(&opts, &meta{})
	fs.VisitAll(func(fl *pflag.Flag) {
		names = append(names, "--"+fl.Name)
		if fl.Shorthand != "" {
			names = append(names, "-"+fl.Shorthand)
		}
	})

	fmt.Printf(`_gofs() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  local opts="%s"
  COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
}
complete -F _gofs gofs
`, strings.Join(names, " "))
}
