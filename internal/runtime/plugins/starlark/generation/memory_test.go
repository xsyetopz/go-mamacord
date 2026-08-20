package generation

import "fmt"

type memoryBundle struct{ sources map[string][]byte }

func (bundle *memoryBundle) ReadSource(label string, maxBytes int64) ([]byte, error) {
	source, ok := bundle.sources[label]
	if !ok {
		return nil, fmt.Errorf("source %q not found", label)
	}
	if int64(len(source)) > maxBytes {
		return nil, fmt.Errorf("source %q is too large", label)
	}
	return append([]byte(nil), source...), nil
}
