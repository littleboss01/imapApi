package api

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/emersion/go-imap/client"
)

const (
	MaxIdleConnections = 5
	MaxOpenConnections = 10
	IdleTimeout        = 30 * time.Second
)

type IMAPPool struct {
	dialer       *net.Dialer
	connections  chan *client.Client
	active       int
	mu           sync.Mutex
	closeChan    chan struct{}
	idleTimeout  time.Duration
	maxOpenConns int
	maxIdleConns int
}

func NewIMAPPool(dialer *net.Dialer) *IMAPPool {
	pool := &IMAPPool{
		dialer:       dialer,
		connections:  make(chan *client.Client, MaxOpenConnections),
		closeChan:    make(chan struct{}),
		idleTimeout:  IdleTimeout, // 超时时间
		maxOpenConns: MaxOpenConnections,
		maxIdleConns: MaxIdleConnections,
	}

	go pool.monitor()

	return pool
}

func (p *IMAPPool) Get(server string) (*client.Client, error) {
	select {
	case conn := <-p.connections:
		return conn, nil
	default:
	}

	p.mu.Lock()
	if p.active >= p.maxOpenConns {
		p.mu.Unlock()
		return nil, ErrPoolExhausted
	}

	conn, err := p.dialer.Dial("tcp", server)
	if err != nil {
		p.mu.Unlock()
		return nil, err
	}

	c, err := client.New(conn)
	if err != nil {
		conn.Close()
		p.mu.Unlock()
		return nil, err
	}

	p.active++
	p.mu.Unlock()

	return c, nil
}

func (p *IMAPPool) Put(c *client.Client) {
	select {
	case p.connections <- c:
		return
	default:
	}

	p.mu.Lock()
	p.active--
	p.mu.Unlock()

	c.Logout()
}

func (p *IMAPPool) monitor() {
	ticker := time.NewTicker(p.idleTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-p.closeChan:
			return
		case <-ticker.C:
			p.mu.Lock()
			if p.active <= p.maxIdleConns {
				p.mu.Unlock()
				continue
			}

			for i := 0; i < p.active-p.maxIdleConns; i++ {
				c := <-p.connections
				p.mu.Unlock()

				c.Logout()
				p.mu.Lock()
				p.active--
				p.mu.Unlock()
			}
			p.mu.Unlock()
		}
	}
}

func (p *IMAPPool) Close() {
	close(p.closeChan)
	p.mu.Lock()
	for c := range p.connections {
		c.Logout()
		p.active--
		if p.active == 0 {
			break
		}
	}
	p.mu.Unlock()
}

var ErrPoolExhausted = errors.New("imap pool exhausted")
