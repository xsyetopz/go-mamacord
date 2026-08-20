package starlark

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	starlarkgo "go.starlark.net/starlark"
)

type memoryBundle struct {
	sources map[string][]byte
	reads   []string
}

func (bundle *memoryBundle) ReadSource(label string, maxBytes int64) ([]byte, error) {
	bundle.reads = append(bundle.reads, label)
	source, exists := bundle.sources[label]
	if !exists {
		return nil, fmt.Errorf("missing %s", label)
	}
	if int64(len(source)) > maxBytes {
		return nil, errors.New("too large")
	}
	return append([]byte(nil), source...), nil
}

func TestCompileAndInitializeControlledModules(t *testing.T) {
	t.Parallel()
	bundle := &memoryBundle{sources: map[string][]byte{
		EntrypointLabel:     []byte("load(\"@mamacord//api.star\", \"identity\")\nload(\"//lib:values.star\", \"VALUES\")\nRESULT = identity(VALUES[0])\nENTRY = []\n"),
		"//lib:values.star": []byte("VALUES = [42]\n"),
	}}
	compiled, err := CompileBundle(context.Background(), bundle, DefaultLimits())
	if err != nil {
		t.Fatalf("CompileBundle: %v", err)
	}
	identity := starlarkgo.NewBuiltin("identity", func(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, _ []starlarkgo.Tuple) (starlarkgo.Value, error) {
		if len(args) != 1 {
			return nil, errors.New("one argument required")
		}
		return args[0], nil
	})
	initialized, err := compiled.Initialize(context.Background(), starlarkgo.StringDict{"identity": identity}, nil)
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if got := initialized.entry["RESULT"]; got.String() != "42" {
		t.Fatalf("RESULT = %v", got)
	}
	entryList := initialized.entry["ENTRY"].(*starlarkgo.List)
	if err := entryList.Append(starlarkgo.MakeInt(1)); err != nil {
		t.Fatalf("entry globals froze before setup: %v", err)
	}
	loadedList := initialized.modules["//lib:values.star"]["VALUES"].(*starlarkgo.List)
	if err := loadedList.Append(starlarkgo.MakeInt(2)); err == nil {
		t.Fatal("loaded module value remained mutable")
	}
	initialized.Freeze()
	if err := entryList.Append(starlarkgo.MakeInt(2)); err == nil {
		t.Fatal("entry globals remained mutable after freeze")
	}
	if len(bundle.reads) != 2 || bundle.reads[0] != EntrypointLabel || bundle.reads[1] != "//lib:values.star" {
		t.Fatalf("reads: %#v", bundle.reads)
	}
}

func TestCompileBundleRejectsGraphViolations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		sources map[string][]byte
		limits  func() Limits
		kind    ErrorKind
	}{
		{name: "relative load", sources: map[string][]byte{EntrypointLabel: []byte("load(\"lib.star\", \"x\")\n")}, limits: DefaultLimits, kind: ErrorLoad},
		{name: "cycle", sources: map[string][]byte{EntrypointLabel: []byte("load(\"//lib:a.star\", \"a\")\n"), "//lib:a.star": []byte("load(\"//lib:b.star\", \"b\")\na = b\n"), "//lib:b.star": []byte("load(\"//lib:a.star\", \"a\")\nb = a\n")}, limits: DefaultLimits, kind: ErrorLoad},
		{name: "module count", sources: map[string][]byte{EntrypointLabel: []byte("load(\"//lib:a.star\", \"a\")\n"), "//lib:a.star": []byte("a = 1\n")}, limits: func() Limits { value := DefaultLimits(); value.MaxModules = 1; return value }, kind: ErrorLoad},
		{name: "load depth", sources: map[string][]byte{EntrypointLabel: []byte("load(\"//lib:a.star\", \"a\")\n"), "//lib:a.star": []byte("a = 1\n")}, limits: func() Limits { value := DefaultLimits(); value.MaxLoadDepth = 1; return value }, kind: ErrorLoad},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := CompileBundle(context.Background(), &memoryBundle{sources: test.sources}, test.limits())
			if err == nil || !IsErrorKind(err, test.kind) {
				t.Fatalf("error = %v, want kind %s", err, test.kind)
			}
		})
	}
}

func TestInitializePreservesLoadErrorKind(t *testing.T) {
	t.Parallel()
	bundle := &memoryBundle{sources: map[string][]byte{
		EntrypointLabel:     []byte("load(\"//lib:broken.star\", \"VALUE\")\n"),
		"//lib:broken.star": []byte("fail(\"broken\")\nVALUE = 1\n"),
	}}
	compiled, err := CompileBundle(context.Background(), bundle, DefaultLimits())
	if err != nil {
		t.Fatalf("CompileBundle: %v", err)
	}
	_, err = compiled.Initialize(context.Background(), nil, nil)
	if err == nil || !IsErrorKind(err, ErrorLoad) {
		t.Fatalf("initialize error = %v, want load", err)
	}
}

func TestInitializeUsesOneFrozenModuleInstance(t *testing.T) {
	t.Parallel()
	bundle := &memoryBundle{sources: map[string][]byte{
		EntrypointLabel:     []byte("load(\"//lib:left.star\", \"LEFT\")\nload(\"//lib:right.star\", \"RIGHT\")\n"),
		"//lib:left.star":   []byte("load(\"//lib:shared.star\", \"VALUE\")\nLEFT = VALUE\n"),
		"//lib:right.star":  []byte("load(\"//lib:shared.star\", \"VALUE\")\nRIGHT = VALUE\n"),
		"//lib:shared.star": []byte("print(\"shared initialized\")\nVALUE = []\n"),
	}}
	compiled, err := CompileBundle(context.Background(), bundle, DefaultLimits())
	if err != nil {
		t.Fatalf("CompileBundle: %v", err)
	}
	var prints []string
	initialized, err := compiled.Initialize(context.Background(), nil, func(message string) { prints = append(prints, message) })
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if len(prints) != 1 || prints[0] != "shared initialized" {
		t.Fatalf("prints: %#v", prints)
	}
	left := initialized.modules["//lib:left.star"]["LEFT"]
	right := initialized.modules["//lib:right.star"]["RIGHT"]
	if left != right {
		t.Fatal("shared module value was initialized more than once")
	}
}

func TestCompileBundleHonorsCancellationAndAggregateBytes(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := CompileBundle(ctx, &memoryBundle{sources: map[string][]byte{EntrypointLabel: []byte("x = 1\n")}}, DefaultLimits())
	if err == nil || !IsErrorKind(err, ErrorCanceled) {
		t.Fatalf("cancellation error: %v", err)
	}

	limits := DefaultLimits()
	limits.MaxFileBytes = 80
	limits.MaxTotalSourceBytes = 100
	main := "load(\"//lib:a.star\", \"a\")\n" + strings.Repeat("#", 40)
	library := "a = 1\n" + strings.Repeat("#", 40)
	_, err = CompileBundle(context.Background(), &memoryBundle{sources: map[string][]byte{EntrypointLabel: []byte(main), "//lib:a.star": []byte(library)}}, limits)
	if err == nil || !IsErrorKind(err, ErrorSource) {
		t.Fatalf("aggregate byte error: %v", err)
	}
}
