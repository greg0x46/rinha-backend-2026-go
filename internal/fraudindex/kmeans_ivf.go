package fraudindex

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"sort"
	"sync"
)

type KMeansBuildOptions struct {
	K          uint32
	Iter       int
	SampleSize int
	Seed       uint64
}

type kmeansEntry struct {
	cluster uint32
	vector  QuantizedVector
	label   Label
}

type lcg struct {
	state uint64
}

func (r *lcg) nextU64() uint64 {
	r.state = r.state*6364136223846793005 + 1442695040888963407
	return r.state
}

func (r *lcg) nextInt(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.nextU64()>>33) % n
}

func (r *lcg) nextFloat64() float64 {
	return float64(r.nextU64()>>11) / float64(uint64(1)<<53)
}

func DefaultKMeansBuildOptions() KMeansBuildOptions {
	return KMeansBuildOptions{
		K:          4096,
		Iter:       20,
		SampleSize: 50000,
		Seed:       0x2026_0bad_cafe_f00d,
	}
}

// kmeansIVFAoS is the array-of-struct layout produced during the build
// phase. It mirrors the on-disk format and is only used to write the
// binary; the in-memory scoring representation (KMeansIVFIndex) uses an
// SoA block layout built at load time.
type kmeansIVFAoS struct {
	Centroids []Vector
	Offsets   []uint64
	Vectors   []QuantizedVector
	Labels    []Label
}

func WriteKMeansIVFBinary(path string, references []Reference, opts KMeansBuildOptions) (Manifest, error) {
	if len(references) == 0 {
		return Manifest{}, errors.New("kmeans ivf references must not be empty")
	}
	aos, err := buildKMeansIVFAoS(references, opts)
	if err != nil {
		return Manifest{}, err
	}
	return writeKMeansIVFAoS(path, aos)
}

func buildKMeansIVFAoS(references []Reference, opts KMeansBuildOptions) (kmeansIVFAoS, error) {
	if opts.K == 0 {
		return kmeansIVFAoS{}, errors.New("kmeans ivf k must be greater than zero")
	}
	if int(opts.K) > len(references) {
		opts.K = uint32(len(references))
	}
	if opts.Iter <= 0 {
		opts.Iter = 1
	}
	if opts.SampleSize <= 0 || opts.SampleSize > len(references) {
		opts.SampleSize = len(references)
	}
	if opts.Seed == 0 {
		opts.Seed = DefaultKMeansBuildOptions().Seed
	}

	sample := sampleReferences(references, opts.SampleSize, opts.Seed)
	centroids := kmeansPlusPlus(sample, int(opts.K), opts.Seed)
	assignments := make([]int, len(sample))
	for iter := 0; iter < opts.Iter; iter++ {
		assignSample(sample, centroids, assignments)
		updateCentroids(sample, assignments, centroids)
	}

	entries := make([]kmeansEntry, len(references))
	centroidCounts := make([]uint64, len(centroids))
	assignReferences(references, centroids, entries, centroidCounts)

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].cluster == entries[j].cluster {
			return i < j
		}
		return entries[i].cluster < entries[j].cluster
	})

	offsets := make([]uint64, len(centroids)+1)
	var total uint64
	for i, count := range centroidCounts {
		offsets[i] = total
		total += count
	}
	offsets[len(centroids)] = total

	vectors := make([]QuantizedVector, len(entries))
	labels := make([]Label, len(entries))
	for i, entry := range entries {
		vectors[i] = entry.vector
		labels[i] = entry.label
	}

	return kmeansIVFAoS{
		Centroids: centroids,
		Offsets:   offsets,
		Vectors:   vectors,
		Labels:    labels,
	}, nil
}

func sampleReferences(references []Reference, sampleSize int, seed uint64) []Vector {
	sample := make([]Vector, sampleSize)
	rng := lcg{state: seed}
	for i := range sample {
		sample[i] = references[rng.nextInt(len(references))].Vector
	}
	return sample
}

func kmeansPlusPlus(sample []Vector, k int, seed uint64) []Vector {
	rng := lcg{state: seed ^ 0xa5a5_a5a5_2026_2026}
	centroids := make([]Vector, 0, k)
	centroids = append(centroids, sample[rng.nextInt(len(sample))])
	minDists := make([]float32, len(sample))
	for i := range minDists {
		minDists[i] = float32(math.Inf(1))
	}
	for len(centroids) < k {
		last := centroids[len(centroids)-1]
		var total float64
		for i := range sample {
			d := squaredDistanceFloat(sample[i], last)
			if d < minDists[i] {
				minDists[i] = d
			}
			total += float64(minDists[i])
		}
		target := rng.nextFloat64() * total
		var acc float64
		chosen := len(sample) - 1
		for i, d := range minDists {
			acc += float64(d)
			if acc >= target {
				chosen = i
				break
			}
		}
		centroids = append(centroids, sample[chosen])
	}
	return centroids
}

func assignSample(sample []Vector, centroids []Vector, assignments []int) {
	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	chunk := (len(sample) + workers - 1) / workers
	var wg sync.WaitGroup
	for start := 0; start < len(sample); start += chunk {
		end := start + chunk
		if end > len(sample) {
			end = len(sample)
		}
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for i := start; i < end; i++ {
				assignments[i] = nearestFloatCentroid(sample[i], centroids)
			}
		}(start, end)
	}
	wg.Wait()
}

func updateCentroids(sample []Vector, assignments []int, centroids []Vector) {
	sums := make([][Dimensions]float64, len(centroids))
	counts := make([]int, len(centroids))
	for i, vector := range sample {
		cluster := assignments[i]
		counts[cluster]++
		for d, value := range vector {
			sums[cluster][d] += float64(value)
		}
	}
	for i := range centroids {
		if counts[i] == 0 {
			continue
		}
		div := float64(counts[i])
		for d := range centroids[i] {
			centroids[i][d] = float32(sums[i][d] / div)
		}
	}
}

func assignReferences(references []Reference, centroids []Vector, entries []kmeansEntry, counts []uint64) {
	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	localCounts := make([][]uint64, workers)
	for i := range localCounts {
		localCounts[i] = make([]uint64, len(centroids))
	}
	chunk := (len(references) + workers - 1) / workers
	var wg sync.WaitGroup
	for worker, start := 0, 0; start < len(references); worker, start = worker+1, start+chunk {
		end := start + chunk
		if end > len(references) {
			end = len(references)
		}
		worker := worker
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			local := localCounts[worker]
			for i := start; i < end; i++ {
				reference := references[i]
				cluster := nearestFloatCentroid(reference.Vector, centroids)
				local[cluster]++
				entries[i] = kmeansEntry{
					cluster: uint32(cluster),
					vector:  QuantizeVector(reference.Vector),
					label:   reference.Label,
				}
			}
		}(start, end)
	}
	wg.Wait()
	for _, local := range localCounts {
		for i, count := range local {
			counts[i] += count
		}
	}
}

func nearestFloatCentroid(vector Vector, centroids []Vector) int {
	best := 0
	bestDistance := float32(math.Inf(1))
	for i, centroid := range centroids {
		distance := squaredDistanceFloat(vector, centroid)
		if distance < bestDistance {
			bestDistance = distance
			best = i
		}
	}
	return best
}

func squaredDistanceFloat(a, b Vector) float32 {
	var distance float32
	for i := range a {
		delta := a[i] - b[i]
		distance += delta * delta
	}
	return distance
}

func writeKMeansIVFAoS(path string, index kmeansIVFAoS) (Manifest, error) {
	if len(index.Offsets) != len(index.Centroids)+1 {
		return Manifest{}, errors.New("kmeans ivf offsets length does not match centroids")
	}
	file, err := os.Create(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("create kmeans ivf binary references: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriterSize(file, 1<<20)
	manifest := Manifest{
		Version:    KMeansIVFBinaryVersion,
		Dimension:  Dimensions,
		References: uint64(len(index.Vectors)),
		Scale:      QuantizationScale,
		NList:      uint32(len(index.Centroids)),
	}
	if _, err := writer.Write(binaryMagic[:]); err != nil {
		return Manifest{}, fmt.Errorf("write kmeans ivf magic: %w", err)
	}
	for _, value := range []any{manifest.Version, manifest.Dimension, manifest.References, manifest.Scale, manifest.NList} {
		if err := binary.Write(writer, binary.LittleEndian, value); err != nil {
			return Manifest{}, fmt.Errorf("write kmeans ivf header: %w", err)
		}
	}
	for _, centroid := range index.Centroids {
		for _, value := range centroid {
			if err := binary.Write(writer, binary.LittleEndian, value); err != nil {
				return Manifest{}, fmt.Errorf("write kmeans ivf centroid: %w", err)
			}
		}
	}
	for _, offset := range index.Offsets {
		if err := binary.Write(writer, binary.LittleEndian, offset); err != nil {
			return Manifest{}, fmt.Errorf("write kmeans ivf offset: %w", err)
		}
	}
	for i, vector := range index.Vectors {
		for _, value := range vector {
			if err := binary.Write(writer, binary.LittleEndian, value); err != nil {
				return Manifest{}, fmt.Errorf("write kmeans ivf vector: %w", err)
			}
		}
		if err := writer.WriteByte(byte(index.Labels[i])); err != nil {
			return Manifest{}, fmt.Errorf("write kmeans ivf label: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return Manifest{}, fmt.Errorf("flush kmeans ivf binary references: %w", err)
	}
	return manifest, nil
}

// BuildBlockedKMeansIVF assembles a blocked KMeansIVFIndex from AoS data
// (the on-disk shape). It is exposed primarily so tests can construct
// indices without round-tripping through a binary file.
func BuildBlockedKMeansIVF(centroids []Vector, offsets []uint64, vectors []QuantizedVector, labels []Label) KMeansIVFIndex {
	nLists := len(centroids)
	blockListOffsets := make([]uint32, nLists+1)
	var totalBlocks uint32
	for i := 0; i < nLists; i++ {
		listSize := uint32(offsets[i+1] - offsets[i])
		blockListOffsets[i] = totalBlocks
		totalBlocks += (listSize + KMeansBlockSize - 1) / KMeansBlockSize
	}
	blockListOffsets[nLists] = totalBlocks

	blocks := make([]int16, int(totalBlocks)*KMeansBlockStride)
	blockLabels := make([]Label, int(totalBlocks)*KMeansBlockSize)

	for i := 0; i < nLists; i++ {
		listStart := int(offsets[i])
		listSize := int(offsets[i+1] - offsets[i])
		blockBase := int(blockListOffsets[i])
		for k := 0; k < listSize; k++ {
			blockIdx := blockBase + k/KMeansBlockSize
			lane := k % KMeansBlockSize
			vec := vectors[listStart+k]
			base := blockIdx * KMeansBlockStride
			for d := 0; d < int(Dimensions); d++ {
				blocks[base+d*KMeansBlockSize+lane] = vec[d]
			}
			blockLabels[blockIdx*KMeansBlockSize+lane] = labels[listStart+k]
		}
	}

	return KMeansIVFIndex{
		Centroids:        centroids,
		Offsets:          offsets,
		BlockListOffsets: blockListOffsets,
		Blocks:           blocks,
		BlockLabels:      blockLabels,
	}
}

func LoadKMeansIVFBinary(path string) (KMeansIVFIndex, Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return KMeansIVFIndex{}, Manifest{}, fmt.Errorf("open kmeans ivf binary references: %w", err)
	}
	defer file.Close()

	manifest, err := readHeader(file)
	if err != nil {
		return KMeansIVFIndex{}, Manifest{}, err
	}
	if manifest.Version != KMeansIVFBinaryVersion {
		return KMeansIVFIndex{}, Manifest{}, fmt.Errorf("binary references version %d is not kmeans ivf", manifest.Version)
	}
	if manifest.Scale != QuantizationScale {
		return KMeansIVFIndex{}, Manifest{}, fmt.Errorf("unsupported quantization scale %d", manifest.Scale)
	}
	if manifest.NList == 0 {
		return KMeansIVFIndex{}, Manifest{}, errors.New("kmeans ivf manifest has zero lists")
	}

	reader := bufio.NewReaderSize(file, 1<<20)
	centroids := make([]Vector, manifest.NList)
	offsets := make([]uint64, int(manifest.NList)+1)
	for i := range centroids {
		for d := range centroids[i] {
			if err := binary.Read(reader, binary.LittleEndian, &centroids[i][d]); err != nil {
				return KMeansIVFIndex{}, Manifest{}, fmt.Errorf("read kmeans ivf centroid %d: %w", i, err)
			}
		}
	}
	for i := range offsets {
		if err := binary.Read(reader, binary.LittleEndian, &offsets[i]); err != nil {
			return KMeansIVFIndex{}, Manifest{}, fmt.Errorf("read kmeans ivf offset %d: %w", i, err)
		}
	}
	if offsets[0] != 0 || offsets[len(offsets)-1] != manifest.References {
		return KMeansIVFIndex{}, Manifest{}, errors.New("kmeans ivf offsets do not match reference count")
	}
	for i := 1; i < len(offsets); i++ {
		if offsets[i] < offsets[i-1] {
			return KMeansIVFIndex{}, Manifest{}, errors.New("kmeans ivf offsets are not monotonic")
		}
	}

	nLists := int(manifest.NList)
	blockListOffsets := make([]uint32, nLists+1)
	var totalBlocks uint32
	for i := 0; i < nLists; i++ {
		listSize := uint32(offsets[i+1] - offsets[i])
		blockListOffsets[i] = totalBlocks
		totalBlocks += (listSize + KMeansBlockSize - 1) / KMeansBlockSize
	}
	blockListOffsets[nLists] = totalBlocks

	blocks := make([]int16, int(totalBlocks)*KMeansBlockStride)
	blockLabels := make([]Label, int(totalBlocks)*KMeansBlockSize)

	listIdx := 0
	for listIdx < nLists && offsets[listIdx] == offsets[listIdx+1] {
		listIdx++
	}

	var record [Dimensions*2 + 1]byte
	var qv QuantizedVector
	for i := uint64(0); i < manifest.References; i++ {
		for listIdx < nLists && i >= offsets[listIdx+1] {
			listIdx++
		}
		if listIdx >= nLists {
			return KMeansIVFIndex{}, Manifest{}, fmt.Errorf("kmeans ivf reference %d falls outside any list", i)
		}
		if _, err := io.ReadFull(reader, record[:]); err != nil {
			return KMeansIVFIndex{}, Manifest{}, fmt.Errorf("read kmeans ivf reference %d: %w", i, err)
		}
		readQuantizedVector(record[:Dimensions*2], &qv)
		label := Label(record[len(record)-1])
		if label != LabelLegit && label != LabelFraud {
			return KMeansIVFIndex{}, Manifest{}, fmt.Errorf("reference %d has invalid label %d", i, label)
		}
		k := uint32(i - offsets[listIdx])
		blockIdx := blockListOffsets[listIdx] + k/KMeansBlockSize
		lane := int(k % KMeansBlockSize)
		base := int(blockIdx) * KMeansBlockStride
		for d := 0; d < int(Dimensions); d++ {
			blocks[base+d*KMeansBlockSize+lane] = qv[d]
		}
		blockLabels[int(blockIdx)*KMeansBlockSize+lane] = label
	}

	var extra [1]byte
	n, err := reader.Read(extra[:])
	if err != nil && !errors.Is(err, io.EOF) {
		return KMeansIVFIndex{}, Manifest{}, fmt.Errorf("check trailing kmeans ivf binary data: %w", err)
	}
	if n != 0 {
		return KMeansIVFIndex{}, Manifest{}, errors.New("kmeans ivf binary references has trailing data")
	}
	return KMeansIVFIndex{
		Centroids:        centroids,
		Offsets:          offsets,
		BlockListOffsets: blockListOffsets,
		Blocks:           blocks,
		BlockLabels:      blockLabels,
	}, manifest, nil
}
