package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func WriteFile(path string, content []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("os.MkdirAll: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, perm); err != nil {
		return fmt.Errorf("os.WriteFile: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("os.Rename: %w", err)
	}

	return nil
}

func Atov(text string) any {
	if text == "null" {
		return nil
	}

	if text == "true" {
		return true
	}

	if text == "false" {
		return false
	}

	if i, err := strconv.ParseInt(text, 10, 64); err == nil {
		return i
	}

	if f, err := strconv.ParseFloat(text, 64); err == nil {
		return f
	}

	trimmed := strings.TrimSpace(text)
	if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
		var v any
		if err := json.Unmarshal([]byte(trimmed), &v); err == nil {
			return v
		}
	}
	return text
}

func Vtoa(value any) string {
	switch val := value.(type) {
	case nil:
		return "(nil)"

	case string:
		return val

	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)

	case bool:
		return strconv.FormatBool(val)

	case map[string]any, []any:
		raw, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(raw)

	default:
		return fmt.Sprintf("%v", val)
	}
}
