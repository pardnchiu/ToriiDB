package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func snapPath(dir string, num int) string {
	return filepath.Join(dir, fmt.Sprintf("%08d.snap", num))
}

func logPath(dir string, num int) string {
	return filepath.Join(dir, fmt.Sprintf("%08d.log", num))
}

func parseName(name string) (int, string, bool) {
	base, kind, ok := strings.Cut(name, ".")
	if !ok || len(base) != 8 {
		return 0, "", false
	}

	if kind != "snap" && kind != "log" {
		return 0, "", false
	}

	num, err := strconv.Atoi(base)
	if err != nil || num < 0 || fmt.Sprintf("%08d", num) != base {
		return 0, "", false
	}

	return num, kind, true
}

func latestSnap(dir string) (string, int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", 0
	}

	latest := -1
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		num, kind, ok := parseName(e.Name())
		if !ok || kind != "snap" {
			continue
		}
		latest = max(latest, num)
	}

	if latest < 0 {
		return "", 0
	}

	return snapPath(dir, latest), latest
}

func gcOlderThan(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		num, kind, ok := parseName(e.Name())
		if !ok {
			continue
		}

		switch kind {
		case "snap":
			if num < keep {
				os.Remove(filepath.Join(dir, e.Name()))
			}
		case "log":
			if num <= keep {
				os.Remove(filepath.Join(dir, e.Name()))
			}
		}
	}
}

func dirExists(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

func sizeOf(path string) int64 {
	if path == "" {
		return 0
	}

	info, err := os.Stat(path)
	if err != nil {
		return 0
	}

	return info.Size()
}
