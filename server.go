package resp

import (
	"errors"
	"net"
	"sync"
)

var DefaultServer = NewServer()

func ListenAndServe(address string) error {
	return DefaultServer.ListenAndServe(address)
}

func AddHandler(method string, fc HandlerFunc) error {
	return DefaultServer.AddHandler(method, fc)
}

type HandlerFunc func(conn *Conn, req *Request)

type Server struct {
	mu       sync.Mutex
	handlers map[string]HandlerFunc
}

func NewServer() *Server {
	return &Server{handlers: map[string]HandlerFunc{}}
}

func (s *Server) AddHandler(method string, fc HandlerFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.handlers == nil {
		s.handlers = map[string]HandlerFunc{}
	}

	if _, ok := s.handlers[method]; ok {
		return errors.New("resp server: handler already set for method: " + method)
	}

	s.handlers[method] = fc
	return nil
}

func (s *Server) ListenAndServe(address string) error {
	l, err := net.Listen("tcp4", address)
	if err != nil {
		return err
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			continue
		}
		go s.Handle(conn)
	}
	return nil
}

func (s *Server) Handle(c net.Conn) {
	defer c.Close()
	conn := NewConn(c)
	for {
		req, err := conn.ReadRequest()
		//协议和请求格式错误直接关闭连接
		if err != nil {
			break
		}
		handler, ok := s.handlers[req.Method]
		if !ok {
			conn.WriteError(errors.New("unsurport method:" + req.Method))
			break
		}
		handler(conn, req)
	}
}
