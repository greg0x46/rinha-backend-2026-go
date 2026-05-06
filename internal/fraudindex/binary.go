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
	BinaryVersion = uint32(1)
	Dimensions    = uint32(14)
)

var binaryMagic = [8]byte{'R', 'B', 'E', '6', 'R', 'E', 'F', '1'}

type Manifest struct {
	Version    uint32
	Dimension  uint32
	References uint64
}

type BinaryWriter struct {
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

func (w *BinaryWriter) Count() uint64 {
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
	if manifest.Version != BinaryVersion {
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
	return manifest, nil
}
