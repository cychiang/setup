//go:build ignore

// tarball writes a reproducible .tar.gz: sorted names, fixed owner, fixed mtime, no gzip timestamp.
// Usage: go run scripts/tarball.go <epoch> <out.tar.gz> <name=path>...
package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tarball:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: tarball <epoch> <out> <name=path>...")
	}
	epoch, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return err
	}
	mtime := time.Unix(epoch, 0)
	entries := args[2:]
	sort.Strings(entries)

	out, err := os.Create(args[1])
	if err != nil {
		return err
	}
	defer out.Close()
	gz, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		name, path, ok := strings.Cut(e, "=")
		if !ok {
			return fmt.Errorf("bad entry %q", e)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		mode := int64(0o644)
		if info.Mode()&0o111 != 0 {
			mode = 0o755
		}
		hdr := &tar.Header{Name: name, Mode: mode, Size: int64(len(data)), ModTime: mtime, Format: tar.FormatUSTAR}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := tw.Write(data); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}
