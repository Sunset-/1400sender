package concurrent

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

//通用连接池

var (
	ErrInvalidConfig = errors.New("invalid pool config")
	ErrPoolClosed    = errors.New("pool closed")
)

type factory func() (io.Closer, error)

type Pool interface {
	Acquire() (io.Closer, error) // 获取资源
	Release(io.Closer) error     // 释放资源
	Remove(io.Closer) error      // 关闭资源
	Close() error                // 关闭池
}

type ConnectPool struct {
	sync.Mutex
	pool     chan io.Closer
	maxOpen  int     // 池中最大资源数
	numOpen  int     // 当前池中资源数
	initOpen int     // 池中最少资源数
	closed   bool    // 池是否已关闭
	factory  factory // 创建连接的方法
}

func NewConnectPool(initOpen, maxOpen int, factory factory) *ConnectPool {
	if initOpen < 0 {
		initOpen = 0
	}
	if maxOpen <= 0 {
		maxOpen = 1
	}
	if maxOpen < initOpen {
		maxOpen = initOpen + 1
	}
	p := &ConnectPool{
		maxOpen:  maxOpen,
		initOpen: initOpen,
		factory:  factory,
		pool:     make(chan io.Closer, maxOpen),
	}

	//初始化连接
	for i := 0; i < initOpen; i++ {
		closer, err := factory()
		if err != nil {
			fmt.Println("初始化连接失败：", err)
			continue
		}
		p.numOpen++
		p.pool <- closer
	}
	return p
}

func (p *ConnectPool) Acquire() (io.Closer, error) {
	if p.closed {
		return nil, ErrPoolClosed
	}
	for {
		closer, err := p.getOrCreate()
		if err != nil {
			return nil, err
		}
		return closer, nil
	}
}

func (p *ConnectPool) getOrCreate() (io.Closer, error) {
	select {
	case closer := <-p.pool:
		return closer, nil
	default:
	}

	p.Lock()
	if p.numOpen < p.maxOpen {
		closer, err := p.factory()
		if err == nil {
			p.numOpen++
		}
		p.Unlock()
		return closer, err
	}
	p.Unlock()
	return <-p.pool, nil
}

// 释放单个资源到连接池
func (p *ConnectPool) Release(closer io.Closer) error {
	p.Lock()
	if p.closed {
		return ErrPoolClosed
	}
	p.pool <- closer
	p.Unlock()
	return nil
}

// 关闭单个资源
func (p *ConnectPool) Remove(closer io.Closer) error {
	p.Lock()
	_ = closer.Close()
	p.numOpen--
	p.Unlock()
	return nil
}

// 关闭连接池，释放所有资源
func (p *ConnectPool) Close() error {
	if p.closed {
		return ErrPoolClosed
	}
	p.Lock()
	close(p.pool)
	for closer := range p.pool {
		_ = closer.Close()
		p.numOpen--
	}
	p.closed = true
	p.Unlock()
	return nil
}
