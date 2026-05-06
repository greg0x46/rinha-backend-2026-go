package fraudindex

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
)

const (
	BinaryVersion          = uint32(1)
	QuantizedBinaryVersion = uint32(2)
	IVFBinaryVersion       = uint32(3)
	Dimensions             = uint32(14)
	QuantizationScale      = uint32(32767)
)

var binaryMagic = [8]byte{'R', 'B', 'E', '6', 'R', 'E', 'F', '1'}

type Manifest struct {
	Version    uint32
	Dimension  uint32
	References uint64
	Scale      uint32
	NList      uint32
}

type BinaryWriter struct {
	writer *bufio.Writer
	file   *os.File
	count  uint64
	closed bool
}

type QuantizedBinaryWriter struct {
	writer *bufio.Writer
	file   *os.File
	count  uint64
	closed bool
}

func CreateBinary(path string) (*BinaryWriter, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create binary references: %w", err)
	}

	writer := bufio.NewWriterSize(file, 1<<20)
	binaryWriter := &BinaryWriter{
		writer: writer,
		file:   file,
	}
	if err := binaryWriter.writeHeader(0); err != nil {
		_ = file.Close()
		return nil, err
	}
	return binaryWriter, nil
}

func CreateQuantizedBinary(path string) (*QuantizedBinaryWriter, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create quantized binary references: %w", err)
	}

	writer := bufio.NewWriterSize(file, 1<<20)
	binaryWriter := &QuantizedBinaryWriter{
		writer: writer,
		file:   file,
	}
	if err := binaryWriter.writeHeader(0); err != nil {
		_ = file.Close()
		return nil, err
	}
	return binaryWriter, nil
}

func (w *BinaryWriter) Write(reference Reference) error {
	if w.closed {
		return errors.New("binary writer is closed")
	}
	for _, value := range reference.Vector {
		if err := binary.Write(w.writer, binary.LittleEndian, value); err != nil {
			return fmt.Errorf("write vector value: %w", err)
		}
	}
	if err := w.writer.WriteByte(byte(reference.Label)); err != nil {
		return fmt.Errorf("write label: %w", err)
	}
	w.count++
	return nil
}

func (w *QuantizedBinaryWriter) Write(reference Reference) error {
	if w.closed {
		return errors.New("quantized binary writer is closed")
	}
	vector := QuantizeVector(reference.Vector)
	for _, value := range vector {
		if err := binary.Write(w.writer, binary.LittleEndian, value); err != nil {
			return fmt.Errorf("write quantized vector value: %w", err)
		}
	}
	if err := w.writer.WriteByte(byte(reference.Label)); err != nil {
		return fmt.Errorf("write label: %w", err)
	}
	w.count++
	return nil
}

func (w *BinaryWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	if err := w.writer.Flush(); err != nil {
		_ = w.file.Close()
		return fmt.Errorf("flush binary references: %w", err)
	}
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		_ = w.file.Close()
		return fmt.Errorf("seek binary references header: %w", err)
	}
	if err := w.writeHeader(w.count); err != nil {
		_ = w.file.Close()
		return err
	}
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("close binary references: %w", err)
	}
	return nil
}

func (w *QuantizedBinaryWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	if err := w.writer.Flush(); err != nil {
		_ = w.file.Close()
		return fmt.Errorf("flush quantized binary references: %w", err)
	}
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		_ = w.file.Close()
		return fmt.Errorf("seek quantized binary references header: %w", err)
	}
	if err := w.writeHeader(w.count); err != nil {
		_ = w.file.Close()
		return err
	}
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("close quantized binary references: %w", err)
	}
	return nil
}

func (w *BinaryWriter) Count() uint64 {
	return w.count
}

func (w *QuantizedBinaryWriter) Count() uint64 {
	return w.count
}

func LoadBinary(path string) ([]Reference, Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, Manifest{}, fmt.Errorf("open binary references: %w", err)
	}
	defer file.Close()

	manifest, err := readHeader(file)
	if err != nil {
		return nil, Manifest{}, err
	}
	if manifest.Version != BinaryVersion {
		return nil, Manifest{}, fmt.Errorf("binary references version %d is not float32", manifest.Version)
	}
	reader := bufio.NewReaderSize(file, 1<<20)

	references := make([]Reference, manifest.References)
	var record [Dimensions*4 + 1]byte
	for i := range references {
		if _, err := io.ReadFull(reader, record[:]); err != nil {
			return nil, Manifest{}, fmt.Errorf("read reference %d: %w", i, err)
		}
		for j := range references[i].Vector {
			offset := j * 4
			references[i].Vector[j] = math.Float32frombits(binary.LittleEndian.Uint32(record[offset : offset+4]))
		}
		references[i].Label = Label(record[len(record)-1])
		if references[i].Label != LabelLegit && references[i].Label != LabelFraud {
			return nil, Manifest{}, fmt.Errorf("reference %d has invalid label %d", i, references[i].Label)
		}
	}

	var extra [1]byte
	n, err := reader.Read(extra[:])
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, Manifest{}, fmt.Errorf("check trailing binary data: %w", err)
	}
	if n != 0 {
		return nil, Manifest{}, errors.New("binary references has trailing data")
	}

	return references, manifest, nil
}

func LoadQuantizedBinary(path string) (QuantizedIndex, Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return QuantizedIndex{}, Manifest{}, fmt.Errorf("open quantized binary references: %w", err)
	}
	defer file.Close()

	manifest, err := readHeader(file)
	if err != nil {
		return QuantizedIndex{}, Manifest{}, err
	}
	if manifest.Version != QuantizedBinaryVersion {
		return QuantizedIndex{}, Manifest{}, fmt.Errorf("binary references version %d is not quantized", manifest.Version)
	}
	if manifest.Scale != QuantizationScale {
		return QuantizedIndex{}, Manifest{}, fmt.Errorf("unsupported quantization scale %d", manifest.Scale)
	}
	reader := bufio.NewReaderSize(file, 1<<20)

	index := QuantizedIndex{
		Vectors: make([]QuantizedVector, manifest.References),
		Labels:  make([]Label, manifest.References),
	}
	var record [Dimensions*2 + 1]byte
	for i := range index.Vectors {
		if _, err := io.ReadFull(reader, record[:]); err != nil {
			return QuantizedIndex{}, Manifest{}, fmt.Errorf("read quantized reference %d: %w", i, err)
		}
		for j := range index.Vectors[i] {
			offset := j * 2
			index.Vectors[i][j] = int16(binary.LittleEndian.Uint16(record[offset : offset+2]))
		}
		index.Labels[i] = Label(record[len(record)-1])
		if index.Labels[i] != LabelLegit && index.Labels[i] != LabelFraud {
			return QuantizedIndex{}, Manifest{}, fmt.Errorf("reference %d has invalid label %d", i, index.Labels[i])
		}
	}

	var extra [1]byte
	n, err := reader.Read(extra[:])
	if err != nil && !errors.Is(err, io.EOF) {
		return QuantizedIndex{}, Manifest{}, fmt.Errorf("check trailing quantized binary data: %w", err)
	}
	if n != 0 {
		return QuantizedIndex{}, Manifest{}, errors.New("quantized binary references has trailing data")
	}

	return index, manifest, nil
}

func WriteIVFBinary(path string, references []Reference, nlist uint32) (Manifest, error) {
	if nlist == 0 {
		return Manifest{}, errors.New("ivf nlist must be greater than zero")
	}
	if len(references) == 0 {
		return Manifest{}, errors.New("ivf references must not be empty")
	}
	if uint64(nlist) > uint64(len(references)) {
		nlist = uint32(len(references))
	}

	index := BuildIVF(references, nlist)
	return writeIVFIndex(path, index)
}

func LoadIVFBinary(path string) (IVFIndex, Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return IVFIndex{}, Manifest{}, fmt.Errorf("open ivf binary references: %w", err)
	}
	defer file.Close()

	manifest, err := readHeader(file)
	if err != nil {
		return IVFIndex{}, Manifest{}, err
	}
	if manifest.Version != IVFBinaryVersion {
		return IVFIndex{}, Manifest{}, fmt.Errorf("binary references version %d is not ivf", manifest.Version)
	}
	if manifest.Scale != QuantizationScale {
		return IVFIndex{}, Manifest{}, fmt.Errorf("unsupported quantization scale %d", manifest.Scale)
	}
	if manifest.NList == 0 {
		return IVFIndex{}, Manifest{}, errors.New("ivf manifest has zero lists")
	}

	reader := bufio.NewReaderSize(file, 1<<20)
	index := IVFIndex{
		Centroids: make([]QuantizedVector, manifest.NList),
		Offsets:   make([]uint64, manifest.NList+1),
		Vectors:   make([]QuantizedVector, manifest.References),
		Labels:    make([]Label, manifest.References),
	}

	var vectorRecord [Dimensions * 2]byte
	for i := range index.Centroids {
		if _, err := io.ReadFull(reader, vectorRecord[:]); err != nil {
			return IVFIndex{}, Manifest{}, fmt.Errorf("read ivf centroid %d: %w", i, err)
		}
		readQuantizedVector(vectorRecord[:], &index.Centroids[i])
	}
	for i := range index.Offsets {
		if err := binary.Read(reader, binary.LittleEndian, &index.Offsets[i]); err != nil {
			return IVFIndex{}, Manifest{}, fmt.Errorf("read ivf offset %d: %w", i, err)
		}
	}
	if index.Offsets[0] != 0 || index.Offsets[len(index.Offsets)-1] != manifest.References {
		return IVFIndex{}, Manifest{}, errors.New("ivf offsets do not match reference count")
	}
	for i := 1; i < len(index.Offsets); i++ {
		if index.Offsets[i] < index.Offsets[i-1] {
			return IVFIndex{}, Manifest{}, errors.New("ivf offsets are not monotonic")
		}
	}

	var record [Dimensions*2 + 1]byte
	for i := range index.Vectors {
		if _, err := io.ReadFull(reader, record[:]); err != nil {
			return IVFIndex{}, Manifest{}, fmt.Errorf("read ivf reference %d: %w", i, err)
		}
		readQuantizedVector(record[:Dimensions*2], &index.Vectors[i])
		index.Labels[i] = Label(record[len(record)-1])
		if index.Labels[i] != LabelLegit && index.Labels[i] != LabelFraud {
			return IVFIndex{}, Manifest{}, fmt.Errorf("reference %d has invalid label %d", i, index.Labels[i])
		}
	}

	var extra [1]byte
	n, err := reader.Read(extra[:])
	if err != nil && !errors.Is(err, io.EOF) {
		return IVFIndex{}, Manifest{}, fmt.Errorf("check trailing ivf binary data: %w", err)
	}
	if n != 0 {
		return IVFIndex{}, Manifest{}, errors.New("ivf binary references has trailing data")
	}

	return index, manifest, nil
}

func BuildIVF(references []Reference, nlist uint32) IVFIndex {
	vectors := make([]QuantizedVector, len(references))
	labels := make([]Label, len(references))
	for i, reference := range references {
		vectors[i] = QuantizeVector(reference.Vector)
		labels[i] = reference.Label
	}
	return BuildQuantizedIVF(vectors, labels, nlist)
}

func BuildQuantizedIVF(vectors []QuantizedVector, labels []Label, nlist uint32) IVFIndex {
	if len(vectors) != len(labels) {
		panic("ivf vectors and labels length mismatch")
	}
	if nlist == 0 {
		panic("ivf nlist must be greater than zero")
	}
	if uint64(nlist) > uint64(len(vectors)) {
		nlist = uint32(len(vectors))
	}

	entries := make([]ivfBuildEntry, len(vectors))
	for i, vector := range vectors {
		entries[i] = ivfBuildEntry{
			key:    projectionKey(vector),
			vector: vector,
			label:  labels[i],
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].key == entries[j].key {
			return i < j
		}
		return entries[i].key < entries[j].key
	})

	centroids := make([]QuantizedVector, nlist)
	offsets := make([]uint64, nlist+1)
	groupedVectors := make([]QuantizedVector, len(vectors))
	groupedLabels := make([]Label, len(labels))
	for list := uint32(0); list < nlist; list++ {
		start := uint64(list) * uint64(len(entries)) / uint64(nlist)
		end := uint64(list+1) * uint64(len(entries)) / uint64(nlist)
		offsets[list] = start
		offsets[list+1] = end

		var sums [Dimensions]int64
		for i := start; i < end; i++ {
			entry := entries[i]
			groupedVectors[i] = entry.vector
			groupedLabels[i] = entry.label
			for dimension, value := range entry.vector {
				sums[dimension] += int64(value)
			}
		}
		count := int64(end - start)
		if count == 0 {
			continue
		}
		for dimension := range centroids[list] {
			centroids[list][dimension] = int16(sums[dimension] / count)
		}
	}

	return IVFIndex{
		Centroids: centroids,
		Offsets:   offsets,
		Vectors:   groupedVectors,
		Labels:    groupedLabels,
	}
}

type ivfBuildEntry struct {
	key    int64
	vector QuantizedVector
	label  Label
}

func QuantizeVector(vector Vector) QuantizedVector {
	var quantized QuantizedVector
	for i, value := range vector {
		if value < -1 {
			value = -1
		} else if value > 1 {
			value = 1
		}
		quantized[i] = int16(math.Round(float64(value * float32(QuantizationScale))))
	}
	return quantized
}

func initialCentroids(vectors []QuantizedVector, nlist uint32) []QuantizedVector {
	centroids := make([]QuantizedVector, nlist)
	if len(vectors) == 0 {
		return centroids
	}
	step := float64(len(vectors)) / float64(nlist)
	for i := range centroids {
		source := int(float64(i) * step)
		if source >= len(vectors) {
			source = len(vectors) - 1
		}
		centroids[i] = vectors[source]
	}
	return centroids
}

func projectionKey(vector QuantizedVector) int64 {
	weights := [...]int64{31, 29, 23, 19, 17, 13, 11, 7, 5, 3, -3, -5, -7, -11}
	var key int64
	for i, value := range vector {
		key += int64(value) * weights[i]
	}
	return key
}

func nearestCentroid(vector QuantizedVector, centroids []QuantizedVector) uint32 {
	best := uint32(0)
	bestDistance := uint64(math.MaxUint64)
	for i, centroid := range centroids {
		distance := SquaredQuantizedDistance(vector, centroid)
		if distance < bestDistance {
			best = uint32(i)
			bestDistance = distance
		}
	}
	return best
}

func SquaredQuantizedDistance(a, b QuantizedVector) uint64 {
	var distance uint64
	for i := range a {
		delta := int64(a[i]) - int64(b[i])
		distance += uint64(delta * delta)
	}
	return distance
}

func readQuantizedVector(record []byte, vector *QuantizedVector) {
	for j := range vector {
		offset := j * 2
		vector[j] = int16(binary.LittleEndian.Uint16(record[offset : offset+2]))
	}
}

func (w *BinaryWriter) writeHeader(count uint64) error {
	if _, err := w.file.Write(binaryMagic[:]); err != nil {
		return fmt.Errorf("write binary magic: %w", err)
	}
	if err := binary.Write(w.file, binary.LittleEndian, BinaryVersion); err != nil {
		return fmt.Errorf("write binary version: %w", err)
	}
	if err := binary.Write(w.file, binary.LittleEndian, Dimensions); err != nil {
		return fmt.Errorf("write binary dimensions: %w", err)
	}
	if err := binary.Write(w.file, binary.LittleEndian, count); err != nil {
		return fmt.Errorf("write binary reference count: %w", err)
	}
	return nil
}

func (w *QuantizedBinaryWriter) writeHeader(count uint64) error {
	if _, err := w.file.Write(binaryMagic[:]); err != nil {
		return fmt.Errorf("write quantized binary magic: %w", err)
	}
	if err := binary.Write(w.file, binary.LittleEndian, QuantizedBinaryVersion); err != nil {
		return fmt.Errorf("write quantized binary version: %w", err)
	}
	if err := binary.Write(w.file, binary.LittleEndian, Dimensions); err != nil {
		return fmt.Errorf("write quantized binary dimensions: %w", err)
	}
	if err := binary.Write(w.file, binary.LittleEndian, count); err != nil {
		return fmt.Errorf("write quantized binary reference count: %w", err)
	}
	if err := binary.Write(w.file, binary.LittleEndian, QuantizationScale); err != nil {
		return fmt.Errorf("write quantized binary scale: %w", err)
	}
	return nil
}

func writeIVFIndex(path string, index IVFIndex) (Manifest, error) {
	if len(index.Offsets) != len(index.Centroids)+1 {
		return Manifest{}, errors.New("ivf offsets length does not match centroids")
	}
	file, err := os.Create(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("create ivf binary references: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriterSize(file, 1<<20)
	manifest := Manifest{
		Version:    IVFBinaryVersion,
		Dimension:  Dimensions,
		References: uint64(len(index.Vectors)),
		Scale:      QuantizationScale,
		NList:      uint32(len(index.Centroids)),
	}

	if _, err := writer.Write(binaryMagic[:]); err != nil {
		return Manifest{}, fmt.Errorf("write ivf magic: %w", err)
	}
	for _, value := range []any{manifest.Version, manifest.Dimension, manifest.References, manifest.Scale, manifest.NList} {
		if err := binary.Write(writer, binary.LittleEndian, value); err != nil {
			return Manifest{}, fmt.Errorf("write ivf header: %w", err)
		}
	}
	for _, centroid := range index.Centroids {
		for _, value := range centroid {
			if err := binary.Write(writer, binary.LittleEndian, value); err != nil {
				return Manifest{}, fmt.Errorf("write ivf centroid: %w", err)
			}
		}
	}
	for _, offset := range index.Offsets {
		if err := binary.Write(writer, binary.LittleEndian, offset); err != nil {
			return Manifest{}, fmt.Errorf("write ivf offset: %w", err)
		}
	}
	for i, vector := range index.Vectors {
		for _, value := range vector {
			if err := binary.Write(writer, binary.LittleEndian, value); err != nil {
				return Manifest{}, fmt.Errorf("write ivf vector: %w", err)
			}
		}
		if err := writer.WriteByte(byte(index.Labels[i])); err != nil {
			return Manifest{}, fmt.Errorf("write ivf label: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return Manifest{}, fmt.Errorf("flush ivf binary references: %w", err)
	}
	return manifest, nil
}

func readHeader(reader io.Reader) (Manifest, error) {
	var magic [8]byte
	if _, err := io.ReadFull(reader, magic[:]); err != nil {
		return Manifest{}, fmt.Errorf("read binary magic: %w", err)
	}
	if magic != binaryMagic {
		return Manifest{}, errors.New("invalid binary references magic")
	}

	var manifest Manifest
	if err := binary.Read(reader, binary.LittleEndian, &manifest.Version); err != nil {
		return Manifest{}, fmt.Errorf("read binary version: %w", err)
	}
	if manifest.Version != BinaryVersion && manifest.Version != QuantizedBinaryVersion && manifest.Version != IVFBinaryVersion {
		return Manifest{}, fmt.Errorf("unsupported binary references version %d", manifest.Version)
	}
	if err := binary.Read(reader, binary.LittleEndian, &manifest.Dimension); err != nil {
		return Manifest{}, fmt.Errorf("read binary dimensions: %w", err)
	}
	if manifest.Dimension != Dimensions {
		return Manifest{}, fmt.Errorf("unsupported binary references dimensions %d", manifest.Dimension)
	}
	if err := binary.Read(reader, binary.LittleEndian, &manifest.References); err != nil {
		return Manifest{}, fmt.Errorf("read binary reference count: %w", err)
	}
	if manifest.Version == QuantizedBinaryVersion || manifest.Version == IVFBinaryVersion {
		if err := binary.Read(reader, binary.LittleEndian, &manifest.Scale); err != nil {
			return Manifest{}, fmt.Errorf("read binary quantization scale: %w", err)
		}
	}
	if manifest.Version == IVFBinaryVersion {
		if err := binary.Read(reader, binary.LittleEndian, &manifest.NList); err != nil {
			return Manifest{}, fmt.Errorf("read binary ivf list count: %w", err)
		}
	}
	return manifest, nil
}
