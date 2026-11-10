package config

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

// helpHeader is the prose part of the help output. The option table below it is
// generated from the FlagSet, so only text that cannot be derived lives here.
const helpHeader = `gofs is a distinctive utility file server.

Usage: gofs [OPTIONS] [serve-path]

Arguments:
  [serve-path]  Specific path to serve [default: .]

Options:
`

// PrintHelp writes the full usage text. The option table comes from the
// registered flags, so adding or renaming one cannot leave the help stale.
func PrintHelp() {
	f := &flags{}
	fs := newFlagSet(f)
	fmt.Print(helpHeader)
	fmt.Print(fs.FlagUsages())
	fmt.Print("\nConfiguration is read from a TOML file (--config), GOFS_* environment\n" +
		"variables and the command line, in increasing order of precedence.\n")
}

// PrintCompletions prints a shell completion script. Only bash is implemented;
// the script itself is generated from the FlagSet for the same reason as help.
func PrintCompletions(shell string) {
	if shell != "bash" {
		fmt.Printf("# completion for %s is not implemented\n", shell)
		return
	}

	// Every flag name, in registration order, plus its shorthand when it has one.
	var opts []string
	f := &flags{}
	fs := newFlagSet(f)
	fs.VisitAll(func(fl *pflag.Flag) {
		opts = append(opts, "--"+fl.Name)
		if fl.Shorthand != "" {
			opts = append(opts, "-"+fl.Shorthand)
		}
	})

	fmt.Printf(`_gofs() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  local opts="%s"
  COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
}
complete -F _gofs gofs
`, strings.Join(opts, " "))
}
