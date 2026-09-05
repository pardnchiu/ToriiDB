package store

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func readGen(dir string) uint64 {
	raw, err := os.ReadFile(filepath.Join(dir, ".gen"))
	if err != nil {
		return 0
	}

	num, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return 0
	}

	return num
}

func bumpGen(dir string) uint64 {
	current := readGen(dir)
	next := current + 1

	tmp := filepath.Join(dir, ".gen.tmp")
	if err := os.WriteFile(tmp, []byte(strconv.FormatUint(next, 10)), 0644); err != nil {
		return current
	}

	if err := os.Rename(tmp, filepath.Join(dir, ".gen")); err != nil {
		os.Remove(tmp)
		return current
	}

	return next
}
