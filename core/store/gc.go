package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
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
	entries, err := go_pkg_filesystem_reader.ListAll(dir)
	if err != nil {
		return "", 0
	}

	latest := -1
	for _, e := range entries {
		num, kind, ok := parseName(e.Name)
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
	entries, err := go_pkg_filesystem_reader.ListAll(dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		num, kind, ok := parseName(e.Name)
		if !ok {
			continue
		}

		switch kind {
		case "snap":
			if num < keep {
				go_pkg_filesystem.Remove(e.Path)
			}
		case "log":
			if num <= keep {
				go_pkg_filesystem.Remove(e.Path)
			}
		}
	}
}

func sizeOf(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}

	return info.Size()
}
