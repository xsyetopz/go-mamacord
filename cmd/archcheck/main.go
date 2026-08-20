package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/xsyetopz/go-mamacord/internal/archcheck"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("archcheck", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "archcheck does not accept positional arguments")
		return 2
	}

	report, err := archcheck.Audit(*root)
	if err != nil {
		fmt.Fprintf(stderr, "archcheck: %v\n", err)
		return 2
	}
	for _, violation := range report.Violations {
		switch violation.Kind {
		case archcheck.CategoricalFiles:
			fmt.Fprintf(stdout, "%s: %d grouped Go files share categorical %s %q (maximum %d before extraction)\n", violation.Path, violation.Count, violation.Axis, violation.Name, violation.Limit)
		case archcheck.RedundantName:
			fmt.Fprintf(stdout, "%s: grouped Go file %q redundantly repeats its package directory name\n", violation.Path, violation.Name)
		case archcheck.FileGroups:
			fmt.Fprintf(stdout, "%s: %d grouped file units (limit %d)\n", violation.Path, violation.Count, violation.Limit)
		case archcheck.StructFields:
			fmt.Fprintf(stdout, "%s: %s has %d fields (limit %d)\n", violation.Path, violation.Name, violation.Count, violation.Limit)
		}
	}
	if !report.OK() {
		return 1
	}
	fmt.Fprintln(stdout, "internal architecture limits: OK")
	return 0
}
