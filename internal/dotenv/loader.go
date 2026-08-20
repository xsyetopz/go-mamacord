package dotenv

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type SearchResult struct {
	Path   string
	Source string
}

// LoadAuto loads the first existing env file from the provided candidates.
// It never overrides already-set environment variables.
//
// Intended usage: call once at process start (before config.LoadFromEnv()).
func LoadAuto(candidates []string) (string, error) {
	res, err := LoadAutoWithSearch(candidates, nil)
	if err != nil {
		return res.Path, err
	}
	return res.Path, nil
}

func LoadAutoWithSearch(candidates []string, searchDirs []SearchResult) (SearchResult, error) {
	for _, cand := range candidates {
		cand = strings.TrimSpace(cand)
		if cand == "" {
			continue
		}
		if filepath.IsAbs(cand) {
			if _, err := os.Stat(cand); err != nil {
				continue
			}
			if err := LoadFile(cand); err != nil {
				return SearchResult{Path: cand, Source: "explicit"}, err
			}
			return SearchResult{Path: cand, Source: "explicit"}, nil
		}
	}
	for _, dir := range searchDirs {
		baseDir := strings.TrimSpace(dir.Path)
		if baseDir == "" {
			continue
		}
		for _, cand := range candidates {
			cand = strings.TrimSpace(cand)
			if cand == "" || filepath.IsAbs(cand) {
				continue
			}
			path := filepath.Join(baseDir, cand)
			if _, err := os.Stat(path); err != nil {
				continue
			}
			if err := LoadFile(path); err != nil {
				return SearchResult{Path: path, Source: dir.Source}, err
			}
			return SearchResult{Path: path, Source: dir.Source}, nil
		}
	}
	for _, cand := range candidates {
		cand = strings.TrimSpace(cand)
		if cand == "" || filepath.IsAbs(cand) {
			continue
		}
		if _, err := os.Stat(cand); err != nil {
			continue
		}
		if err := LoadFile(cand); err != nil {
			return SearchResult{Path: cand, Source: "working_dir"}, err
		}
		return SearchResult{Path: cand, Source: "working_dir"}, nil
	}
	return SearchResult{}, nil
}

// LoadFile parses a simple KEY=VALUE env file and sets variables that are not
// already present in the environment.
//
// Supported:
// - comments with leading '#'
// - blank lines
// - optional "export " prefix
// - quoted values with '...' or "..."
func LoadFile(path string) error {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	// Allow moderately long lines without pulling in a more complex parser.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 256*1024)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, val, ok, err := parseAssignment(line)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, val); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return nil
}

func parseAssignment(line string) (key string, value string, ok bool, err error) {
	idx := strings.IndexByte(line, '=')
	if idx < 0 {
		// Not an assignment; ignore.
		return "", "", false, nil
	}
	key = strings.TrimSpace(line[:idx])
	if key == "" {
		return "", "", false, errors.New("invalid dotenv line: empty key")
	}
	value = strings.TrimSpace(line[idx+1:])
	value = stripInlineComment(value)
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
	}
	return key, value, true, nil
}

func stripInlineComment(value string) string {
	inSingle := false
	inDouble := false
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return value[:i]
			}
		}
	}
	return value
}
