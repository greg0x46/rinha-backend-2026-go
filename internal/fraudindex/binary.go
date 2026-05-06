package fraudindex

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
)

const (
	BinaryVersion          = uint32(1)
	QuantizedBinaryVersion = uint32(2)
	Dimensions             = uint32(14)
	QuantizationScale      = uint32(32767)
)

var binaryMagic = [8]byte{'R', 'B', 'E', '6', 'R', 'E', 'F', '1'}

type Manifest struct {
	Version    uint32
	Dimension  uint32
	References uint64
	Scale      uint32
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
	if manifest.Version != BinaryVersion && manifest.Version != QuantizedBinaryVersion {
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
	if manifest.Version == QuantizedBinaryVersion {
		if err := binary.Read(reader, binary.LittleEndian, &manifest.Scale); err != nil {
			return Manifest{}, fmt.Errorf("read binary quantization scale: %w", err)
		}
	}
	return manifest, nil
}
