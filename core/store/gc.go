package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

func snapNumbers(dir string) []int {
	entries, err := go_pkg_filesystem_reader.ListAll(dir)
	if err != nil {
		return nil
	}

	var nums []int
	for _, e := range entries {
		num, kind, ok := parseName(e.Name)
		if !ok || kind != "snap" {
			continue
		}
		nums = append(nums, num)
	}

	sort.Sort(sort.Reverse(sort.IntSlice(nums)))
	return nums
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
			if num < keep-1 {
				go_pkg_filesystem.Remove(e.Path)
			}
		case "log":
			if num <= keep-1 {
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
