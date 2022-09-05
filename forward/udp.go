package forward

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

const (
	ttl           = time.Second * 30
	connChanLen   = 128
	maxBufferSize = 8 * 1024
)

var (
	udpConnMap      = newAddrMap()
	ErrConnExist    = errors.New("connection exists")
	ErrConnNotExist = errors.New("connection does not exist")
)

// addrMap
type addrMap struct {
	lock sync.Mutex
	cmap map[string]*udpConn
	size int
}

func newAddrMap() *addrMap {
	return &addrMap{
		lock: sync.Mutex{},
		cmap: make(map[string]*udpConn),
		size: 0,
	}
}

func (m *addrMap) has(key string) bool {
	_, ok := m.cmap[key]
	return ok
}

func (m *addrMap) Load(key string) (*udpConn, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if !m.has(key) {
		return nil, ErrConnNotExist
	}
	return m.cmap[key], nil
}

func (m *addrMap) Store(key string, conn *udpConn) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if !m.has(key) {
		m.size += 1
	}
	m.cmap[key] = conn
}

func (m *addrMap) Delete(key string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if !m.has(key) {
		return
	}
	delete(m.cmap, key)
	m.size -= 1
}

func (m *addrMap) Clear(f func(*udpConn)) {
	m.lock.Lock()
	defer m.lock.Unlock()
	for key, conn := range m.cmap {
		delete(m.cmap, key)
		m.size -= 1
		if f != nil {
			f(conn)
		}
	}
}

func (m *addrMap) Size() int {
	return m.size
}

// udpConn
// udpConn implements net.Conn interface
type udpConn struct {
	lnConn     net.PacketConn
	raddr      net.Addr
	rChan      chan []byte
	keepAlive  chan bool
	isClosed   bool
	muClosed   *sync.Mutex
	chanClosed *chan bool
}

func (c *udpConn) keepAliveTimer(duration time.Duration) {
	if c.IsClosed() {
		return
	}

	timer := time.NewTimer(duration)
	for {
		select {
		case <-*c.chanClosed:
			return
		case <-c.keepAlive:
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(duration)
		case <-timer.C:
			c.Close()
			return
		}
	}
}

func (c *udpConn) readFrom(b []byte) (int, net.Addr, error) {
	if c.IsClosed() {
		return 0, nil, errors.New("connection closed")
	}

	n := 0
	select {
	case <-*c.chanClosed:
		return 0, nil, io.EOF
	case p := <-c.rChan:
		n = copy(b, p)
	}

	if n > 0 {
		c.keepAlive <- true
	}
	return n, c.raddr, nil
}

//
// This function does not return on c.Close();
// And thus, c.Write does not return on c.Close();
// The main reason is that c.lnConn.WriteTo does not
// allow external interrupt
//
func (c *udpConn) writeTo(b []byte, addr net.Addr) (int, error) {
	if c.IsClosed() {
		return 0, errors.New("connection closed")
	}

	n, err := c.lnConn.WriteTo(b, addr)
	if n > 0 {
		c.keepAlive <- true
	}
	return n, err
}

func (c *udpConn) IsClosed() bool {
	c.muClosed.Lock()
	defer c.muClosed.Unlock()
	closed := c.isClosed
	return closed
}

func (c *udpConn) Read(b []byte) (int, error) {
	n, _, err := c.readFrom(b)
	return n, err
}

func (c *udpConn) Write(b []byte) (int, error) {
	return c.writeTo(b, c.raddr)
}

//
// Called when c.lnConn is closed or c is timed out
//
func (c *udpConn) Close() error {
	c.muClosed.Lock()
	defer c.muClosed.Unlock()
	if c.isClosed {
		return errors.New("connection is already closed")
	}

	c.isClosed = true
	close(*c.chanClosed)

	map_key := c.raddr.String()
	udpConnMap.Delete(map_key)
	return nil
}

func (c *udpConn) LocalAddr() net.Addr {
	return c.lnConn.LocalAddr()
}

func (c *udpConn) RemoteAddr() net.Addr {
	return c.raddr
}

//
// This function is here to make udpConn a net.Conn only
// and might have concurrent issue if called
//
func (c *udpConn) SetDeadline(t time.Time) error {
	return c.lnConn.SetDeadline(t)
}

//
// This function is here to make udpConn a net.Conn only
// and might have concurrent issue if called
//
func (c *udpConn) SetReadDeadline(t time.Time) error {
	return c.lnConn.SetReadDeadline(t)
}

//
// This function is here to make udpConn a net.Conn only
// and might have concurrent issue if called
//
func (c *udpConn) SetWriteDeadline(t time.Time) error {
	return c.lnConn.SetWriteDeadline(t)
}

// Listener
type UDPListener struct {
	lnConn   net.PacketConn
	connChan chan *udpConn
	ttl      time.Duration
	isClosed bool
	muClosed *sync.Mutex
}

func (ln *UDPListener) newUdpConn(raddr net.Addr) *udpConn {
	rChan := make(chan []byte, 16)
	keepAlive := make(chan bool, 16)
	muClosed := sync.Mutex{}
	chanClosed := make(chan bool)

	conn := udpConn{
		lnConn:     ln.lnConn,
		raddr:      raddr,
		rChan:      rChan,
		keepAlive:  keepAlive,
		isClosed:   false,
		muClosed:   &muClosed,
		chanClosed: &chanClosed,
	}

	go conn.keepAliveTimer(ln.ttl)
	return &conn
}

func (ln *UDPListener) listenLoop() {
	for {
		buf := make([]byte, maxBufferSize)
		n, addr, err := ln.lnConn.ReadFrom(buf)
		if err != nil {
			ln.Close()
			return
		}
		map_key := addr.String()
		conn, err := udpConnMap.Load(map_key)
		if err != nil {
			conn = ln.newUdpConn(addr)
			udpConnMap.Store(map_key, conn)
		}
		ln.connChan <- conn
		conn.rChan <- buf[:n]
	}
}

func (ln *UDPListener) Close() error {
	ln.muClosed.Lock()
	defer ln.muClosed.Unlock()
	if ln.isClosed {
		return errors.New("listener is already closed")
	}
	ln.isClosed = true
	close(ln.connChan)
	ln.lnConn.Close()
	udpConnMap.Clear(
		func(conn *udpConn) {
			if !conn.IsClosed() {
				conn.Close()
			}
		},
	)
	return nil
}

func (ln *UDPListener) Addr() net.Addr {
	return ln.lnConn.LocalAddr()
}

func (ln *UDPListener) Accept() (net.Conn, error) {
	conn, ok := <-ln.connChan
	if !ok {
		return nil, errors.New("connection closed")
	}
	return conn, nil
}

// Handler
type UDPHandler struct {
	targetAddr    string
	targetUDPAdrr *net.UDPAddr
}

func (h *UDPHandler) Handle(conn net.Conn) error {
	defer conn.Close()

	agent, err := net.DialUDP("udp", nil, h.targetUDPAdrr)
	if err != nil {
		return err
	}
	defer agent.Close()

	return copy_io(conn, agent)
}

// Factory functions
func NewUDPListener(listenAddr string) (Listener, error) {
	lnConn, err := net.ListenPacket("udp", listenAddr)
	if err != nil {
		return nil, err
	}
	connChan := make(chan *udpConn, connChanLen)
	muClosed := sync.Mutex{}
	ln := &UDPListener{
		lnConn:   lnConn,
		connChan: connChan,
		ttl:      ttl,
		isClosed: false,
		muClosed: &muClosed,
	}
	go ln.listenLoop()
	// go func() {
	// 	for {
	// 		time.Sleep(5 * time.Second)
	// 		fmt.Printf("active connection: %d\n", udpConnMap.Size())
	// 	}
	// }()
	return ln, nil
}

func NewUDPHandler(targetAddr string) (Handler, error) {
	targetUDPAdrr, err := net.ResolveUDPAddr("udp", targetAddr)
	if err != nil {
		return nil, err
	}
	handler := &UDPHandler{
		targetAddr:    targetAddr,
		targetUDPAdrr: targetUDPAdrr,
	}
	return handler, nil
}

func NewUDPServer(listenAddr string, targetAddr string) (*Server, error) {
	listener, err := NewUDPListener(listenAddr)
	if err != nil {
		return nil, err
	}
	handler, err := NewUDPHandler(targetAddr)
	if err != nil {
		listener.Close()
		return nil, err
	}
	return &Server{listener, handler}, nil
}
