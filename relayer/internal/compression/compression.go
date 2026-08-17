package compression

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"sync"
)

type Compressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
	Name() string
}

type GzipCompressor struct {
	level int
	mu    sync.RWMutex
}

func NewGzipCompressor(level ...int) *GzipCompressor {
	l := gzip.DefaultCompression
	if len(level) > 0 {
		l = level[0]
	}
	return &GzipCompressor{level: l}
}

func (gc *GzipCompressor) Compress(data []byte) ([]byte, error) {
	gc.mu.RLock()
	level := gc.level
	gc.mu.RUnlock()

	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, level)
	if err != nil {
		return nil, err
	}

	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (gc *GzipCompressor) Decompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return io.ReadAll(r)
}

func (gc *GzipCompressor) Name() string { return "gzip" }

func (gc *GzipCompressor) SetLevel(level int) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.level = level
}

type SnappyCompressor struct {
	mu sync.RWMutex
}

func NewSnappyCompressor() *SnappyCompressor {
	return &SnappyCompressor{}
}

func (sc *SnappyCompressor) Compress(data []byte) ([]byte, error) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}

func (sc *SnappyCompressor) Decompress(data []byte) ([]byte, error) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}

func (sc *SnappyCompressor) Name() string { return "snappy" }

type LZ4Compressor struct {
	level int
	mu    sync.RWMutex
}

func NewLZ4Compressor(level ...int) *LZ4Compressor {
	l := 1
	if len(level) > 0 {
		l = level[0]
	}
	return &LZ4Compressor{level: l}
}

func (lc *LZ4Compressor) Compress(data []byte) ([]byte, error) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	result := make([]byte, len(data)+4)
	result[0] = byte(lc.level)
	copy(result[1:], data)
	return result, nil
}

func (lc *LZ4Compressor) Decompress(data []byte) ([]byte, error) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	if len(data) < 2 {
		return nil, fmt.Errorf("invalid lz4 data")
	}
	result := make([]byte, len(data)-1)
	copy(result, data[1:])
	return result, nil
}

func (lc *LZ4Compressor) Name() string { return "lz4" }

type ZstdCompressor struct {
	level int
	mu    sync.RWMutex
}

func NewZstdCompressor(level ...int) *ZstdCompressor {
	l := 3
	if len(level) > 0 {
		l = level[0]
	}
	return &ZstdCompressor{level: l}
}

func (zc *ZstdCompressor) Compress(data []byte) ([]byte, error) {
	zc.mu.RLock()
	defer zc.mu.RUnlock()
	result := make([]byte, len(data)+1)
	result[0] = byte(zc.level)
	copy(result[1:], data)
	return result, nil
}

func (zc *ZstdCompressor) Decompress(data []byte) ([]byte, error) {
	zc.mu.RLock()
	defer zc.mu.RUnlock()
	if len(data) < 2 {
		return nil, fmt.Errorf("invalid zstd data")
	}
	result := make([]byte, len(data)-1)
	copy(result, data[1:])
	return result, nil
}

func (zc *ZstdCompressor) Name() string { return "zstd" }

type CompressorRegistry struct {
	compressors map[string]Compressor
	mu          sync.RWMutex
}

func NewCompressorRegistry() *CompressorRegistry {
	return &CompressorRegistry{
		compressors: make(map[string]Compressor),
	}
}

func (cr *CompressorRegistry) Register(c Compressor) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.compressors[c.Name()] = c
}

func (cr *CompressorRegistry) Get(name string) Compressor {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	return cr.compressors[name]
}

func (cr *CompressorRegistry) List() []string {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	names := make([]string, 0, len(cr.compressors))
	for name := range cr.compressors {
		names = append(names, name)
	}
	return names
}

type AdaptiveCompressor struct {
	thresholds []CompressThreshold
	mu         sync.RWMutex
}

type CompressThreshold struct {
	DataSize    int
	Compressor  Compressor
}

func NewAdaptiveCompressor(thresholds []CompressThreshold) *AdaptiveCompressor {
	return &AdaptiveCompressor{thresholds: thresholds}
}

func (ac *AdaptiveCompressor) Compress(data []byte) ([]byte, error) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	for _, t := range ac.thresholds {
		if len(data) <= t.DataSize {
			return t.Compressor.Compress(data)
		}
	}

	if len(ac.thresholds) > 0 {
		return ac.thresholds[len(ac.thresholds)-1].Compressor.Compress(data)
	}

	return data, nil
}

func (ac *AdaptiveCompressor) Decompress(data []byte) ([]byte, error) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	if len(ac.thresholds) > 0 {
		return ac.thresholds[0].Compressor.Decompress(data)
	}
	return data, nil
}

func (ac *AdaptiveCompressor) Name() string { return "adaptive" }

type StreamingCompressor struct {
	inner   Compressor
	bufSize int
	mu      sync.RWMutex
}

func NewStreamingCompressor(inner Compressor, bufSize int) *StreamingCompressor {
	if bufSize <= 0 {
		bufSize = 4096
	}
	return &StreamingCompressor{
		inner:   inner,
		bufSize: bufSize,
	}
}

func (sc *StreamingCompressor) CompressReader(r io.Reader) ([]byte, error) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	buf := make([]byte, 0, sc.bufSize)
	tmp := make([]byte, sc.bufSize)

	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	return sc.inner.Compress(buf)
}

func (sc *StreamingCompressor) DecompressReader(data []byte) (io.Reader, error) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	result, err := sc.inner.Decompress(data)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(result), nil
}

type CompressResult struct {
	Original    []byte
	Compressed  []byte
	Ratio       float64
	Compressor  string
}

func CompressWithStats(c Compressor, data []byte) (*CompressResult, error) {
	compressed, err := c.Compress(data)
	if err != nil {
		return nil, err
	}

	ratio := float64(len(compressed)) / float64(len(data))
	return &CompressResult{
		Original:   data,
		Compressed: compressed,
		Ratio:      ratio,
		Compressor: c.Name(),
	}, nil
}

type BatchCompressor struct {
	compressor Compressor
	mu         sync.RWMutex
}

func NewBatchCompressor(compressor Compressor) *BatchCompressor {
	return &BatchCompressor{compressor: compressor}
}

func (bc *BatchCompressor) CompressBatch(datas [][]byte) ([][]byte, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	results := make([][]byte, len(datas))
	for i, data := range datas {
		compressed, err := bc.compressor.Compress(data)
		if err != nil {
			return nil, fmt.Errorf("failed to compress item %d: %w", i, err)
		}
		results[i] = compressed
	}
	return results, nil
}

func (bc *BatchCompressor) DecompressBatch(datas [][]byte) ([][]byte, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	results := make([][]byte, len(datas))
	for i, data := range datas {
		decompressed, err := bc.compressor.Decompress(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress item %d: %w", i, err)
		}
		results[i] = decompressed
	}
	return results, nil
}

type CompressMiddleware struct {
	compressor Compressor
	threshold  int
	mu         sync.RWMutex
}

func NewCompressMiddleware(compressor Compressor, threshold int) *CompressMiddleware {
	return &CompressMiddleware{
		compressor: compressor,
		threshold:  threshold,
	}
}

func (cm *CompressMiddleware) MaybeCompress(data []byte) ([]byte, bool, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if len(data) < cm.threshold {
		return data, false, nil
	}

	compressed, err := cm.compressor.Compress(data)
	if err != nil {
		return nil, false, err
	}

	if len(compressed) < len(data) {
		return compressed, true, nil
	}
	return data, false, nil
}

func (cm *CompressMiddleware) SetThreshold(threshold int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.threshold = threshold
}
