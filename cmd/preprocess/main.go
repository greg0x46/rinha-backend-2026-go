package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/greg/rinha-be-2026/internal/fraudindex"
)

func main() {
	input := flag.String("input", "data/references.json.gz", "JSON or JSON gzip references input")
	output := flag.String("output", "data/references.bin", "binary references output")
	expect := flag.Uint64("expect", 3_000_000, "expected reference count; set 0 to skip validation")
	flag.Parse()

	started := time.Now()
	count, err := preprocess(*input, *output, *expect)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("wrote %d references to %s in %s\n", count, *output, time.Since(started).Round(time.Millisecond))
}

func preprocess(inputPath, outputPath string, expected uint64) (uint64, error) {
	input, err := os.Open(inputPath)
	if err != nil {
		return 0, fmt.Errorf("open input: %w", err)
	}
	defer input.Close()

	reader, closeReader, err := referenceReader(inputPath, input)
	if err != nil {
		return 0, err
	}
	defer closeReader()

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return 0, fmt.Errorf("create output directory: %w", err)
	}

	tmpPath := outputPath + ".tmp"
	writer, err := fraudindex.CreateBinary(tmpPath)
	if err != nil {
		return 0, err
	}

	count, streamErr := fraudindex.StreamJSONReferences(reader, writer.Write)
	closeErr := writer.Close()
	if streamErr != nil {
		_ = os.Remove(tmpPath)
		return count, streamErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return count, closeErr
	}
	if expected != 0 && count != expected {
		_ = os.Remove(tmpPath)
		return count, fmt.Errorf("reference count = %d, want %d", count, expected)
	}
	if err := os.Rename(tmpPath, outputPath); err != nil {
		_ = os.Remove(tmpPath)
		return count, fmt.Errorf("replace output: %w", err)
	}

	return count, nil
}

func referenceReader(path string, reader io.Reader) (io.Reader, func(), error) {
	if !strings.HasSuffix(path, ".gz") {
		return reader, func() {}, nil
	}

	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("open gzip reader: %w", err)
	}
	return gzipReader, func() { _ = gzipReader.Close() }, nil
}
