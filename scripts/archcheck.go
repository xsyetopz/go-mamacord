package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	MaxFileGroups           = 10
	MaxStructFields         = 10
	MaxCategoricalFileGroup = 1
)

type Kind string

const (
	CategoricalFiles Kind = "categorical_files"
	FileGroups       Kind = "file_groups"
	RedundantName    Kind = "redundant_name"
	StructFields     Kind = "struct_fields"
)

type Violation struct {
	Kind  Kind
	Path  string
	Name  string
	Axis  string
	Count int
	Limit int
}

type categoryKey struct {
	axis string
	name string
}

type Report struct {
	Violations []Violation
}

func (r Report) OK() bool {
	return len(r.Violations) == 0
}

func Audit(root string) (Report, error) {
	internalRoot := filepath.Join(root, "internal")
	if info, err := os.Stat(internalRoot); err != nil {
		return Report{}, fmt.Errorf("stat internal directory: %w", err)
	} else if !info.IsDir() {
		return Report{}, fmt.Errorf("internal path %q is not a directory", internalRoot)
	}

	var report Report
	err := filepath.WalkDir(internalRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}

		violations, err := auditDirectory(root, path)
		if err != nil {
			return err
		}
		report.Violations = append(report.Violations, violations...)
		return nil
	})
	if err != nil {
		return Report{}, fmt.Errorf("audit internal architecture: %w", err)
	}

	sort.Slice(report.Violations, func(i, j int) bool {
		left, right := report.Violations[i], report.Violations[j]
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		return left.Name < right.Name
	})
	return report, nil
}

func auditDirectory(root, dir string) ([]Violation, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %q: %w", dir, err)
	}

	groups := make(map[string]struct{})
	goGroups := make(map[string]struct{})
	var violations []Violation
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		group := conceptualGroup(entry.Name())
		groups[group] = struct{}{}
		if filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		goGroups[group] = struct{}{}
		path := filepath.Join(dir, entry.Name())
		structViolations, err := auditStructs(root, path)
		if err != nil {
			return nil, err
		}
		violations = append(violations, structViolations...)
	}

	relativeDir := relativePath(root, dir)
	categories := categoricalGroups(goGroups)
	for key, categoryGroups := range categories {
		if len(categoryGroups) <= MaxCategoricalFileGroup {
			continue
		}
		violations = append(violations, Violation{
			Kind:  CategoricalFiles,
			Path:  relativeDir,
			Name:  key.name,
			Axis:  key.axis,
			Count: len(categoryGroups),
			Limit: MaxCategoricalFileGroup,
		})
	}

	directoryName := strings.ReplaceAll(filepath.Base(dir), "-", "_")
	compactDirectoryName := strings.ReplaceAll(directoryName, "_", "")
	for group := range goGroups {
		compactGroup := strings.ReplaceAll(group, "_", "")
		if group == directoryName || compactGroup == compactDirectoryName || strings.HasPrefix(group, directoryName+"_") || strings.HasSuffix(group, "_"+directoryName) {
			violations = append(violations, Violation{
				Kind: RedundantName,
				Path: relativeDir,
				Name: group,
			})
		}
	}

	if len(groups) > MaxFileGroups {
		violations = append(violations, Violation{
			Kind:  FileGroups,
			Path:  relativeDir,
			Count: len(groups),
			Limit: MaxFileGroups,
		})
	}
	return violations, nil
}

func categoricalGroups(goGroups map[string]struct{}) map[categoryKey][]string {
	sets := make(map[categoryKey]map[string]struct{})
	for group := range goGroups {
		parts := strings.Split(group, "_")
		if len(parts) < 2 {
			continue
		}
		for _, key := range []categoryKey{{axis: "prefix", name: parts[0]}, {axis: "suffix", name: parts[len(parts)-1]}} {
			if key.name == "" {
				continue
			}
			if sets[key] == nil {
				sets[key] = make(map[string]struct{})
			}
			sets[key][group] = struct{}{}
			if _, exists := goGroups[key.name]; exists {
				sets[key][key.name] = struct{}{}
			}
		}
	}

	categories := make(map[categoryKey][]string)
	for key, groups := range sets {
		for group := range groups {
			categories[key] = append(categories[key], group)
		}
		sort.Strings(categories[key])
	}
	return categories
}

func auditStructs(root, path string) ([]Violation, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}

	var violations []Violation
	ast.Inspect(file, func(node ast.Node) bool {
		typeSpec, ok := node.(*ast.TypeSpec)
		if !ok {
			return true
		}
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return true
		}

		count := 0
		for _, field := range structType.Fields.List {
			if len(field.Names) == 0 {
				count++
			} else {
				count += len(field.Names)
			}
		}
		if count > MaxStructFields {
			position := fileSet.Position(typeSpec.Pos())
			violations = append(violations, Violation{
				Kind:  StructFields,
				Path:  fmt.Sprintf("%s:%d", relativePath(root, path), position.Line),
				Name:  typeSpec.Name.Name,
				Count: count,
				Limit: MaxStructFields,
			})
		}
		return false
	})
	return violations, nil
}

func conceptualGroup(name string) string {
	extension := filepath.Ext(name)
	stem := strings.TrimSuffix(name, extension)
	if extension == ".go" {
		return strings.TrimSuffix(stem, "_test")
	}
	return stem
}

func relativePath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

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

	report, err := Audit(*root)
	if err != nil {
		fmt.Fprintf(stderr, "archcheck: %v\n", err)
		return 2
	}
	for _, violation := range report.Violations {
		switch violation.Kind {
		case CategoricalFiles:
			fmt.Fprintf(stdout, "%s: %d grouped Go files share categorical %s %q (maximum %d before extraction)\n", violation.Path, violation.Count, violation.Axis, violation.Name, violation.Limit)
		case RedundantName:
			fmt.Fprintf(stdout, "%s: grouped Go file %q redundantly repeats its package directory name\n", violation.Path, violation.Name)
		case FileGroups:
			fmt.Fprintf(stdout, "%s: %d grouped file units (limit %d)\n", violation.Path, violation.Count, violation.Limit)
		case StructFields:
			fmt.Fprintf(stdout, "%s: %s has %d fields (limit %d)\n", violation.Path, violation.Name, violation.Count, violation.Limit)
		}
	}
	if !report.OK() {
		return 1
	}
	fmt.Fprintln(stdout, "internal architecture limits: OK")
	return 0
}
