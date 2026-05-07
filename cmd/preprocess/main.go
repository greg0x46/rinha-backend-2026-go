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
	format := flag.String("format", "int16", "binary format: int16, ivf-int16, kmeans-ivf-int16, or float32")
	expect := flag.Uint64("expect", 3_000_000, "expected reference count; set 0 to skip validation")
	nlist := flag.Uint("nlist", 1024, "IVF list count when format=ivf-int16 or kmeans-ivf-int16")
	kmeansIter := flag.Int("kmeans-iter", 20, "kmeans iterations when format=kmeans-ivf-int16")
	kmeansSample := flag.Int("kmeans-sample", 50000, "kmeans sample size when format=kmeans-ivf-int16")
	kmeansSeed := flag.Uint64("kmeans-seed", 0x20260badcafef00d, "kmeans seed when format=kmeans-ivf-int16")
	flag.Parse()

	started := time.Now()
	count, err := preprocess(*input, *output, *format, *expect, uint32(*nlist), *kmeansIter, *kmeansSample, *kmeansSeed)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("wrote %d %s references to %s in %s\n", count, *format, *output, time.Since(started).Round(time.Millisecond))
}

func preprocess(inputPath, outputPath, format string, expected uint64, nlist uint32, kmeansIter, kmeansSample int, kmeansSeed uint64) (uint64, error) {
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
	if format == "ivf-int16" || format == "kmeans-ivf-int16" {
		references, err := fraudindex.LoadJSONReferencesFromReader(reader)
		if err != nil {
			_ = os.Remove(tmpPath)
			return 0, err
		}
		count := uint64(len(references))
		if expected != 0 && count != expected {
			_ = os.Remove(tmpPath)
			return count, fmt.Errorf("reference count = %d, want %d", count, expected)
		}
		if format == "ivf-int16" {
			if _, err := fraudindex.WriteIVFBinary(tmpPath, references, nlist); err != nil {
				_ = os.Remove(tmpPath)
				return count, err
			}
		} else {
			opts := fraudindex.KMeansBuildOptions{
				K:          nlist,
				Iter:       kmeansIter,
				SampleSize: kmeansSample,
				Seed:       kmeansSeed,
			}
			if _, err := fraudindex.WriteKMeansIVFBinary(tmpPath, references, opts); err != nil {
				_ = os.Remove(tmpPath)
				return count, err
			}
		}
		if err := os.Rename(tmpPath, outputPath); err != nil {
			_ = os.Remove(tmpPath)
			return count, fmt.Errorf("replace output: %w", err)
		}
		return count, nil
	}

	writer, err := createWriter(tmpPath, format)
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

type referenceWriter interface {
	Write(fraudindex.Reference) error
	Close() error
}

func createWriter(path, format string) (referenceWriter, error) {
	switch format {
	case "float32":
		return fraudindex.CreateBinary(path)
	case "int16":
		return fraudindex.CreateQuantizedBinary(path)
	default:
		return nil, fmt.Errorf("unsupported binary format %q", format)
	}
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
