package transport

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type Transport interface {
	Dial(addr string) (Connection, error)
	Listen(addr string) (Listener, error)
	Name() string
}

type Connection interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
	RemoteAddr() net.Addr
	LocalAddr() net.Addr
	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

type Listener interface {
	Accept() (Connection, error)
	Close() error
	Addr() net.Addr
}

type HTTPTransport struct {
	clientTimeout time.Duration
	transport     *httpTransportImpl
	mu            sync.RWMutex
}

type httpTransportImpl struct {
	maxIdleConns        int
	maxIdleConnsPerHost int
	idleConnTimeout     time.Duration
}

func NewHTTPTransport(timeout time.Duration) *HTTPTransport {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &HTTPTransport{
		clientTimeout: timeout,
		transport: &httpTransportImpl{
			maxIdleConns:        100,
			maxIdleConnsPerHost: 10,
			idleConnTimeout:     90 * time.Second,
		},
	}
}

func (ht *HTTPTransport) Dial(addr string) (Connection, error) {
	conn, err := net.DialTimeout("tcp", addr, ht.clientTimeout)
	if err != nil {
		return nil, fmt.Errorf("http dial failed: %w", err)
	}
	return &tcpConn{conn: conn}, nil
}

func (ht *HTTPTransport) Listen(addr string) (Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("http listen failed: %w", err)
	}
	return &tcpListener{listener: ln}, nil
}

func (ht *HTTPTransport) Name() string { return "http" }

type TCPTransport struct {
	dialTimeout  time.Duration
	connTimeout  time.Duration
	keepAlive    time.Duration
	mu           sync.RWMutex
}

func NewTCPTransport(dialTimeout, connTimeout time.Duration) *TCPTransport {
	if dialTimeout <= 0 {
		dialTimeout = 10 * time.Second
	}
	if connTimeout <= 0 {
		connTimeout = 30 * time.Second
	}
	return &TCPTransport{
		dialTimeout: dialTimeout,
		connTimeout: connTimeout,
		keepAlive:   30 * time.Second,
	}
}

func (tt *TCPTransport) Dial(addr string) (Connection, error) {
	conn, err := net.DialTimeout("tcp", addr, tt.dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("tcp dial failed: %w", err)
	}
	return &tcpConn{conn: conn}, nil
}

func (tt *TCPTransport) Listen(addr string) (Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("tcp listen failed: %w", err)
	}
	return &tcpListener{listener: ln}, nil
}

func (tt *TCPTransport) Name() string { return "tcp" }

type UDPTransport struct {
	dialTimeout time.Duration
	mu          sync.RWMutex
}

func NewUDPTransport(dialTimeout time.Duration) *UDPTransport {
	if dialTimeout <= 0 {
		dialTimeout = 5 * time.Second
	}
	return &UDPTransport{dialTimeout: dialTimeout}
}

func (ut *UDPTransport) Dial(addr string) (Connection, error) {
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return nil, fmt.Errorf("udp dial failed: %w", err)
	}
	return &udpConn{conn: conn}, nil
}

func (ut *UDPTransport) Listen(addr string) (Listener, error) {
	laddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return nil, fmt.Errorf("udp listen failed: %w", err)
	}
	return &udpListener{conn: conn}, nil
}

func (ut *UDPTransport) Name() string { return "udp" }

type WebSocketTransport struct {
	mu sync.RWMutex
}

func NewWebSocketTransport() *WebSocketTransport {
	return &WebSocketTransport{}
}

func (wst *WebSocketTransport) Dial(addr string) (Connection, error) {
	return nil, fmt.Errorf("websocket transport: not implemented")
}

func (wst *WebSocketTransport) Listen(addr string) (Listener, error) {
	return nil, fmt.Errorf("websocket transport: not implemented")
}

func (wst *WebSocketTransport) Name() string { return "websocket" }

type GRPCTransport struct {
	mu sync.RWMutex
}

func NewGRPCTransport() *GRPCTransport {
	return &GRPCTransport{}
}

func (gt *GRPCTransport) Dial(addr string) (Connection, error) {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("grpc dial failed: %w", err)
	}
	return &tcpConn{conn: conn}, nil
}

func (gt *GRPCTransport) Listen(addr string) (Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("grpc listen failed: %w", err)
	}
	return &tcpListener{listener: ln}, nil
}

func (gt *GRPCTransport) Name() string { return "grpc" }

type TransportFactory struct {
	transports map[string]Transport
	mu         sync.RWMutex
}

func NewTransportFactory() *TransportFactory {
	factory := &TransportFactory{
		transports: make(map[string]Transport),
	}
	factory.Register(NewTCPTransport(10*time.Second, 30*time.Second))
	factory.Register(NewUDPTransport(5*time.Second))
	factory.Register(NewHTTPTransport(30 * time.Second))
	return factory
}

func (tf *TransportFactory) Register(transport Transport) {
	tf.mu.Lock()
	defer tf.mu.Unlock()
	tf.transports[transport.Name()] = transport
}

func (tf *TransportFactory) Get(name string) Transport {
	tf.mu.RLock()
	defer tf.mu.RUnlock()
	return tf.transports[name]
}

func (tf *TransportFactory) List() []string {
	tf.mu.RLock()
	defer tf.mu.RUnlock()
	names := make([]string, 0, len(tf.transports))
	for name := range tf.transports {
		names = append(names, name)
	}
	return names
}

type ConnectionPool struct {
	transport Transport
	conns     chan Connection
	maxSize   int
	addr      string
	mu        sync.RWMutex
	closed    bool
}

func NewConnectionPool(transport Transport, addr string, maxSize int) *ConnectionPool {
	if maxSize <= 0 {
		maxSize = 10
	}
	return &ConnectionPool{
		transport: transport,
		conns:     make(chan Connection, maxSize),
		maxSize:   maxSize,
		addr:      addr,
	}
}

func (cp *ConnectionPool) Get() (Connection, error) {
	cp.mu.RLock()
	closed := cp.closed
	cp.mu.RUnlock()

	if closed {
		return nil, fmt.Errorf("pool closed")
	}

	select {
	case conn := <-cp.conns:
		return conn, nil
	default:
		return cp.transport.Dial(cp.addr)
	}
}

func (cp *ConnectionPool) Put(conn Connection) {
	cp.mu.RLock()
	closed := cp.closed
	cp.mu.RUnlock()

	if closed {
		conn.Close()
		return
	}

	select {
	case cp.conns <- conn:
	default:
		conn.Close()
	}
}

func (cp *ConnectionPool) Close() {
	cp.mu.Lock()
	cp.closed = true
	cp.mu.Unlock()

	close(cp.conns)
	for conn := range cp.conns {
		conn.Close()
	}
}

type MiddlewareChain struct {
	middlewares []func(Connection) Connection
	mu          sync.RWMutex
}

func NewMiddlewareChain() *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: make([]func(Connection) Connection, 0),
	}
}

func (mc *MiddlewareChain) Use(mw func(Connection) Connection) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.middlewares = append(mc.middlewares, mw)
}

func (mc *MiddlewareChain) Wrap(conn Connection) Connection {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := conn
	for i := len(mc.middlewares) - 1; i >= 0; i-- {
		result = mc.middlewares[i](result)
	}
	return result
}

type ConnectionManager struct {
	conns map[string]Connection
	mu    sync.RWMutex
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		conns: make(map[string]Connection),
	}
}

func (cm *ConnectionManager) Add(id string, conn Connection) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.conns[id] = conn
}

func (cm *ConnectionManager) Get(id string) Connection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.conns[id]
}

func (cm *ConnectionManager) Remove(id string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if conn, exists := cm.conns[id]; exists {
		conn.Close()
		delete(cm.conns, id)
	}
}

func (cm *ConnectionManager) CloseAll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	for id, conn := range cm.conns {
		conn.Close()
		delete(cm.conns, id)
	}
}

func (cm *ConnectionManager) Count() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.conns)
}

type tcpConn struct {
	conn net.Conn
}

func (c *tcpConn) Read(p []byte) (int, error)                 { return c.conn.Read(p) }
func (c *tcpConn) Write(p []byte) (int, error)                { return c.conn.Write(p) }
func (c *tcpConn) Close() error                               { return c.conn.Close() }
func (c *tcpConn) RemoteAddr() net.Addr                       { return c.conn.RemoteAddr() }
func (c *tcpConn) LocalAddr() net.Addr                        { return c.conn.LocalAddr() }
func (c *tcpConn) SetDeadline(t time.Time) error              { return c.conn.SetDeadline(t) }
func (c *tcpConn) SetReadDeadline(t time.Time) error          { return c.conn.SetReadDeadline(t) }
func (c *tcpConn) SetWriteDeadline(t time.Time) error         { return c.conn.SetWriteDeadline(t) }

type udpConn struct {
	conn *net.UDPConn
}

func (c *udpConn) Read(p []byte) (int, error)                 { return c.conn.Read(p) }
func (c *udpConn) Write(p []byte) (int, error)                { return c.conn.Write(p) }
func (c *udpConn) Close() error                               { return c.conn.Close() }
func (c *udpConn) RemoteAddr() net.Addr                       { return c.conn.RemoteAddr() }
func (c *udpConn) LocalAddr() net.Addr                        { return c.conn.LocalAddr() }
func (c *udpConn) SetDeadline(t time.Time) error              { return c.conn.SetDeadline(t) }
func (c *udpConn) SetReadDeadline(t time.Time) error          { return c.conn.SetReadDeadline(t) }
func (c *udpConn) SetWriteDeadline(t time.Time) error         { return c.conn.SetWriteDeadline(t) }

type tcpListener struct {
	listener net.Listener
}

func (l *tcpListener) Accept() (Connection, error) {
	conn, err := l.listener.Accept()
	if err != nil {
		return nil, err
	}
	return &tcpConn{conn: conn}, nil
}

func (l *tcpListener) Close() error    { return l.listener.Close() }
func (l *tcpListener) Addr() net.Addr  { return l.listener.Addr() }

type udpListener struct {
	conn *net.UDPConn
}

func (l *udpListener) Accept() (Connection, error) {
	buf := make([]byte, 65535)
	n, raddr, err := l.conn.ReadFromUDP(buf)
	if err != nil {
		return nil, err
	}
	return &udpReadConn{data: buf[:n], raddr: raddr, conn: l.conn}, nil
}

func (l *udpListener) Close() error   { return l.conn.Close() }
func (l *udpListener) Addr() net.Addr { return l.conn.LocalAddr() }

type udpReadConn struct {
	data  []byte
	raddr *net.UDPAddr
	conn  *net.UDPConn
	off   int
}

func (c *udpReadConn) Read(p []byte) (int, error) {
	if c.off >= len(c.data) {
		return 0, io.EOF
	}
	n := copy(p, c.data[c.off:])
	c.off += n
	return n, nil
}

func (c *udpReadConn) Write(p []byte) (int, error)                { return c.conn.WriteToUDP(p, c.raddr) }
func (c *udpReadConn) Close() error                               { return nil }
func (c *udpReadConn) RemoteAddr() net.Addr                       { return c.raddr }
func (c *udpReadConn) LocalAddr() net.Addr                        { return c.conn.LocalAddr() }
func (c *udpReadConn) SetDeadline(t time.Time) error              { return nil }
func (c *udpReadConn) SetReadDeadline(t time.Time) error          { return nil }
func (c *udpReadConn) SetWriteDeadline(t time.Time) error         { return nil }
