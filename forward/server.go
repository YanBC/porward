package forward

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
)

// Listener
type Listener interface {
	net.Listener
}

// Handler
type Handler interface {
	Handle(conn net.Conn) error
}

func copy_io(conn1, conn2 net.Conn) error {
	err_chan := make(chan error, 1)
	go func() {
		nbytes, err := io.Copy(conn1, conn2)
		log.Printf("%s -> %s: %d bytes", conn2.RemoteAddr(), conn1.RemoteAddr(), nbytes)
		err_chan <- err
	}()

	go func() {
		nbytes, err := io.Copy(conn2, conn1)
		log.Printf("%s -> %s: %d bytes", conn1.RemoteAddr(), conn2.RemoteAddr(), nbytes)
		err_chan <- err
	}()

	err := <-err_chan
	if err != nil && err == io.EOF {
		err = nil
	}
	return err
}

// Server
type Server struct {
	listener Listener
	handler  Handler
}

func (s *Server) Close() error {
	return s.listener.Close()
}

func (s *Server) Addr() net.Addr {
	return s.listener.Addr()
}

func (s *Server) Serve() {
	sig_chan := make(chan os.Signal, 1)
	signal.Notify(sig_chan, os.Interrupt)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-sig_chan:
			default:
				log.Printf("error accepting connection: %s", err)
			}
			break
		}
		log.Printf("accepted connection: %s", conn.RemoteAddr())

		go func(client *net.Conn) {
			err := s.handler.Handle(*client)
			var log_message string
			if err != nil {
				log_message = fmt.Sprintf("%s [ERROR]: %s", log_message, err)
			} else {
				log_message = fmt.Sprintf("client %s disconnected", (*client).RemoteAddr())
			}
			log.Println(log_message)
		}(&conn)
	}
}
