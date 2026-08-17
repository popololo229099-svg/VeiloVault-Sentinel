package transport

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type TLSTransport struct {
	certFile string
	keyFile  string
	mu       sync.RWMutex
}

func NewTLSTransport(certFile, keyFile string) *TLSTransport {
	return &TLSTransport{
		certFile: certFile,
		keyFile:  keyFile,
	}
}

func (tt *TLSTransport) Dial(addr string) (Connection, error) {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("tls dial failed: %w", err)
	}
	return &tcpConn{conn: conn}, nil
}

func (tt *TLSTransport) Listen(addr string) (Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("tls listen failed: %w", err)
	}
	return &tcpListener{listener: ln}, nil
}

func (tt *TLSTransport) Name() string { return "tls" }

type EncryptedTransport struct {
	inner    Transport
	key      []byte
	gcm      cipher.AEAD
	mu       sync.RWMutex
}

func NewEncryptedTransport(inner Transport, key []byte) (*EncryptedTransport, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &EncryptedTransport{
		inner: inner,
		key:   key,
		gcm:   gcm,
	}, nil
}

func (et *EncryptedTransport) Dial(addr string) (Connection, error) {
	return et.inner.Dial(addr)
}

func (et *EncryptedTransport) Listen(addr string) (Listener, error) {
	return et.inner.Listen(addr)
}

func (et *EncryptedTransport) Name() string { return "encrypted_" + et.inner.Name() }

func (et *EncryptedTransport) Encrypt(data []byte) ([]byte, error) {
	et.mu.RLock()
	defer et.mu.RUnlock()

	nonce := make([]byte, et.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return et.gcm.Seal(nonce, nonce, data, nil), nil
}

func (et *EncryptedTransport) Decrypt(data []byte) ([]byte, error) {
	et.mu.RLock()
	defer et.mu.RUnlock()

	nonceSize := et.gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return et.gcm.Open(nil, nonce, ciphertext, nil)
}

type MultiplexTransport struct {
	inner     Transport
	streams   map[string]Connection
	mu        sync.RWMutex
}

func NewMultiplexTransport(inner Transport) *MultiplexTransport {
	return &MultiplexTransport{
		inner:   inner,
		streams: make(map[string]Connection),
	}
}

func (mt *MultiplexTransport) Dial(addr string) (Connection, error) {
	mt.mu.RLock()
	if conn, ok := mt.streams[addr]; ok {
		mt.mu.RUnlock()
		return conn, nil
	}
	mt.mu.RUnlock()

	conn, err := mt.inner.Dial(addr)
	if err != nil {
		return nil, err
	}

	mt.mu.Lock()
	mt.streams[addr] = conn
	mt.mu.Unlock()

	return conn, nil
}

func (mt *MultiplexTransport) Listen(addr string) (Listener, error) {
	return mt.inner.Listen(addr)
}

func (mt *MultiplexTransport) Name() string { return "multiplex_" + mt.inner.Name() }

func (mt *MultiplexTransport) CloseStream(addr string) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	if conn, ok := mt.streams[addr]; ok {
		conn.Close()
		delete(mt.streams, addr)
	}
}

func (mt *MultiplexTransport) CloseAll() {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	for addr, conn := range mt.streams {
		conn.Close()
		delete(mt.streams, addr)
	}
}

type RateLimitedTransport struct {
	inner     Transport
	maxPerSec int
	current   int
	lastReset time.Time
	mu        sync.Mutex
}

func NewRateLimitedTransport(inner Transport, maxPerSec int) *RateLimitedTransport {
	if maxPerSec <= 0 {
		maxPerSec = 100
	}
	return &RateLimitedTransport{
		inner:     inner,
		maxPerSec: maxPerSec,
		lastReset: time.Now(),
	}
}

func (rlt *RateLimitedTransport) Dial(addr string) (Connection, error) {
	rlt.mu.Lock()
	now := time.Now()
	if now.Sub(rlt.lastReset) >= time.Second {
		rlt.current = 0
		rlt.lastReset = now
	}
	rlt.current++
	if rlt.current > rlt.maxPerSec {
		rlt.mu.Unlock()
		return nil, fmt.Errorf("rate limit exceeded")
	}
	rlt.mu.Unlock()

	return rlt.inner.Dial(addr)
}

func (rlt *RateLimitedTransport) Listen(addr string) (Listener, error) {
	return rlt.inner.Listen(addr)
}

func (rlt *RateLimitedTransport) Name() string { return "ratelimited_" + rlt.inner.Name() }

type BufferedConnection struct {
	inner     Connection
	readBuf   []byte
	writeBuf  []byte
	readPos   int
	writePos  int
	readSize  int
	writeSize int
	mu        sync.RWMutex
}

func NewBufferedConnection(conn Connection, readSize, writeSize int) *BufferedConnection {
	if readSize <= 0 {
		readSize = 4096
	}
	if writeSize <= 0 {
		writeSize = 4096
	}
	return &BufferedConnection{
		inner:     conn,
		readBuf:   make([]byte, readSize),
		writeBuf:  make([]byte, writeSize),
		readSize:  readSize,
		writeSize: writeSize,
	}
}

func (bc *BufferedConnection) Read(p []byte) (int, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if bc.readPos >= bc.readSize {
		n, err := bc.inner.Read(bc.readBuf)
		bc.readPos = 0
		bc.readSize = n
		if err != nil {
			return 0, err
		}
	}

	n := copy(p, bc.readBuf[bc.readPos:bc.readSize])
	bc.readPos += n
	return n, nil
}

func (bc *BufferedConnection) Write(p []byte) (int, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	written := 0
	for written < len(p) {
		n := copy(bc.writeBuf[bc.writePos:], p[written:])
		bc.writePos += n
		written += n

		if bc.writePos >= bc.writeSize {
			_, err := bc.inner.Write(bc.writeBuf[:bc.writePos])
			bc.writePos = 0
			if err != nil {
				return written, err
			}
		}
	}
	return written, nil
}

func (bc *BufferedConnection) Flush() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if bc.writePos > 0 {
		_, err := bc.inner.Write(bc.writeBuf[:bc.writePos])
		bc.writePos = 0
		return err
	}
	return nil
}

func (bc *BufferedConnection) Close() error {
	bc.Flush()
	return bc.inner.Close()
}

func (bc *BufferedConnection) RemoteAddr() net.Addr { return bc.inner.RemoteAddr() }
func (bc *BufferedConnection) LocalAddr() net.Addr  { return bc.inner.LocalAddr() }

func (bc *BufferedConnection) SetDeadline(t time.Time) error {
	return bc.inner.SetDeadline(t)
}

func (bc *BufferedConnection) SetReadDeadline(t time.Time) error {
	return bc.inner.SetReadDeadline(t)
}

func (bc *BufferedConnection) SetWriteDeadline(t time.Time) error {
	return bc.inner.SetWriteDeadline(t)
}

type ConnectionMonitor struct {
	conns    map[string]*ConnectionStats
	mu       sync.RWMutex
}

type ConnectionStats struct {
	BytesRead    int64
	BytesWritten int64
	Reads        int64
	Writes       int64
	Errors       int64
	ConnectedAt  time.Time
	LastActivity time.Time
}

func NewConnectionMonitor() *ConnectionMonitor {
	return &ConnectionMonitor{
		conns: make(map[string]*ConnectionStats),
	}
}

func (cm *ConnectionMonitor) Track(id string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.conns[id] = &ConnectionStats{
		ConnectedAt: time.Now(),
	}
}

func (cm *ConnectionMonitor) RecordRead(id string, bytes int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if stats, ok := cm.conns[id]; ok {
		stats.BytesRead += int64(bytes)
		stats.Reads++
		stats.LastActivity = time.Now()
	}
}

func (cm *ConnectionMonitor) RecordWrite(id string, bytes int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if stats, ok := cm.conns[id]; ok {
		stats.BytesWritten += int64(bytes)
		stats.Writes++
		stats.LastActivity = time.Now()
	}
}

func (cm *ConnectionMonitor) RecordError(id string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if stats, ok := cm.conns[id]; ok {
		stats.Errors++
	}
}

func (cm *ConnectionMonitor) GetStats(id string) *ConnectionStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	stats, ok := cm.conns[id]
	if !ok {
		return nil
	}
	result := *stats
	return &result
}

func (cm *ConnectionMonitor) Remove(id string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.conns, id)
}

func (cm *ConnectionMonitor) Snapshot() map[string]*ConnectionStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make(map[string]*ConnectionStats, len(cm.conns))
	for k, v := range cm.conns {
		cp := *v
		result[k] = &cp
	}
	return result
}

type Base64Transport struct {
	inner Transport
	mu    sync.RWMutex
}

func NewBase64Transport(inner Transport) *Base64Transport {
	return &Base64Transport{inner: inner}
}

func (bt *Base64Transport) Dial(addr string) (Connection, error) {
	return bt.inner.Dial(addr)
}

func (bt *Base64Transport) Listen(addr string) (Listener, error) {
	return bt.inner.Listen(addr)
}

func (bt *Base64Transport) Name() string { return "base64_" + bt.inner.Name() }

func (bt *Base64Transport) Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func (bt *Base64Transport) Decode(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}
