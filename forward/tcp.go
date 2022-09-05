package forward

import (
	"net"
)

type TCPHandler struct {
	targetAddr string
}

func (h *TCPHandler) Handle(conn net.Conn) error {
	defer conn.Close()

	agent, err := net.Dial("tcp", h.targetAddr)
	if err != nil {
		return err
	}
	defer agent.Close()

	return copy_io(agent, conn)
}

func NewTCPListener(listenAddr string) (Listener, error) {
	return net.Listen("tcp", listenAddr)
}

func NewTCPHandler(targetAddr string) Handler {
	return &TCPHandler{targetAddr}
}

func NewTCPServer(listenAddr string, targetAddr string) (*Server, error) {
	listener, err := NewTCPListener(listenAddr)
	if err != nil {
		return nil, err
	}
	handler := NewTCPHandler(targetAddr)
	return &Server{listener, handler}, nil
}
