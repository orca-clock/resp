package resp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
)

type Conn struct {
	conn net.Conn
	mu   sync.Mutex
	br   *bufio.Reader
	bw   *bufio.Writer
}

func NewConn(c net.Conn) *Conn {
	return &Conn{
		conn: c,
		br:   bufio.NewReader(c),
		bw:   bufio.NewWriter(c),
	}
}

type Request struct {
	Method string
	Args   []interface{}
}

func (conn *Conn) ReadRequest() (*Request, error) {
	reply, err := conn.ReadReply()

	if err != nil {
		return nil, err
	}

	args, ok := reply.([]interface{})
	if !ok {
		return nil, errors.New("bad request")
	}

	if len(args) == 0 {
		return nil, errors.New("bad request")
	}

	method, ok := args[0].(string)
	if !ok {
		return nil, errors.New("bad request")
	}

	method = strings.ToLower(method)

	req := &Request{method, args[1:]}
	return req, nil
}

func (c *Conn) ReadReply() (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.readReply()
}

func (c *Conn) WriteStatus(reply string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	defer c.bw.Flush()
	c.bw.WriteString("+")
	c.bw.WriteString(reply)
	_, err := c.bw.WriteString("\r\n")
	return err
}

func (c *Conn) WriteInteger(reply int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	defer c.bw.Flush()
	c.bw.WriteString(":")
	c.bw.WriteString(strconv.Itoa(reply))
	_, err := c.bw.WriteString("\r\n")
	return err
}

func (c *Conn) WriteError(reply error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	defer c.bw.Flush()
	c.bw.WriteString("-")
	c.bw.WriteString(reply.Error())
	_, err := c.bw.WriteString("\r\n")
	return err
}

func (c *Conn) WriteNil() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	defer c.bw.Flush()
	return c.writeNil()
}

func (c *Conn) writeNil() error {
	_, err := c.bw.WriteString("$-1\r\n")
	return err
}

func (c *Conn) writeString(reply string) error {
	c.writeLen("$", len([]byte(reply)))
	_, err := c.bw.WriteString(reply + "\r\n")
	return err
}

func (c *Conn) writeBytes(reply []byte) error {
	c.writeLen("$", len(reply))
	_, err := c.bw.Write(reply)
	return err
}

func (c *Conn) writeLen(head string, n int) error {
	c.bw.WriteString(head)
	c.bw.WriteString(strconv.Itoa(n))
	_, err := c.bw.WriteString("\r\n")
	return err
}

func (c *Conn) WriteBulk(reply interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	defer c.bw.Flush()
	return c.writeBulk(reply)
}
func (c *Conn) writeBulk(reply interface{}) error {
	switch reply := reply.(type) {
	case nil:
		c.WriteNil()
	case string:
		c.writeString(reply)
	case []byte:
		c.writeString(string(reply))
	case int:
		c.writeString(strconv.FormatInt(int64(reply), 10))
	case int64:
		c.writeString(strconv.FormatInt(reply, 10))
	case float32:
		c.writeString(strconv.FormatFloat(float64(reply), 'g', 10, 64))
	case float64:
		c.writeString(strconv.FormatFloat(reply, 'g', 10, 64))
	case bool:
		if bool(reply) {
			c.writeString("1")
		} else {
			c.writeString("0")
		}
	default:
		var buf bytes.Buffer
		fmt.Fprint(&buf, reply)
		return c.writeBytes(buf.Bytes())
	}
	return nil
}

func (c *Conn) WriteReply(args []interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	defer c.bw.Flush()
	c.writeLen("*", len(args))
	var err error
	for _, arg := range args {
		err = c.writeBulk(arg)
		if err != nil {
			break
		}
	}

	return err
}

// return int64|string|nil|[]interface{}
func (c *Conn) readReply() (interface{}, error) {
	line, isPrefix, err := c.br.ReadLine()
	if err != nil {
		return nil, err
	}

	if isPrefix {
		return nil, errors.New("bad protocol")
	}

	switch line[0] {
	case '+':
		return string(line[1:]), nil
	case '-':
		return errors.New(string(line[1:])), nil
	case ':':
		return strconv.ParseInt(string(line[1:]), 10, 0)
	case '$':
		n, err := strconv.ParseInt(string(line[1:]), 10, 0)
		if err != nil {
			return nil, errors.New("bad protocal")
		}

		if n == -1 {
			return nil, nil
		}

		buf := make([]byte, n)
		_, err = io.ReadFull(c.br, buf)
		if err != nil {
			return nil, err
		}

		line, _, err = c.br.ReadLine()
		if err != nil {
			return nil, err
		}
		if len(line) > 0 {
			return nil, errors.New("bad protocol")
		}
		return string(buf), nil
	case '*':
		n, err := strconv.ParseInt(string(line[1:]), 10, 0)
		if err != nil {
			return nil, errors.New("bad protocol")
		}

		if n < 0 { // -1
			return nil, nil
		}

		rts := make([]interface{}, 0, n)
		var i int64
		for i = 0; i < n; i++ {
			rt, err := c.readReply()
			if err != nil {
				return nil, err
			}
			rts = append(rts, rt)
		}

		return rts, nil
	}
	return nil, errors.New("bad protocol")
}
