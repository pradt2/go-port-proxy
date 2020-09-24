package connectivity

import (
	"net"
	"io"
	"sync"
	"project-proxy/logs"
)

var log = logs.GetLoggerForModule("proxy")

type proxy struct {
	connA net.Conn
	connB net.Conn
	onFinished func(connA net.Conn, connB net.Conn)
}

type ConnProxy interface {
	Run()
	RunAsync()
	Stop()
	SetOnFinishedListener(onFinished func(connA net.Conn, connB net.Conn))
}

func NewConnProxy(connA net.Conn, connB net.Conn) ConnProxy {
	return &proxy{
		connA: connA,
		connB: connB,
		onFinished: nil,
	}
}

func (p *proxy) Run() {
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		continuousBufferCopy(p.connA, p.connB)
		wg.Done()
	}()
	go func () {
		continuousBufferCopy(p.connB, p.connA)
		wg.Done()
	}()
	wg.Wait()
	p.Stop()
	if p.onFinished != nil {
		p.onFinished(p.connA, p.connB)
	}
}

func (p *proxy) RunAsync() {
	go p.Run()
}

func (p *proxy) Stop() {
	err := p.connA.Close()
	if err != nil {
		log.Noticef("Could not close connection while stopping the proxy. Proxy will be stopped regardless. Cause: %s", err)
	}
	err = p.connB.Close()
	if err != nil {
		log.Noticef("Could not close connection while stopping the proxy. Proxy will be stopped regardless. Cause: %s", err)
	}
}

func (p *proxy) SetOnFinishedListener(onFinished func(connA net.Conn, connB net.Conn)) {
	p.onFinished = onFinished
}

func continuousBufferCopy(src net.Conn, dest net.Conn) error {
	defer func() {
		if r := recover(); r != nil {
			log.Warningf("Buffer copying ended with a panic which was recovered. Cause: %s", r)
		}
	}()
	len, err := io.Copy(dest, src)
	if err != nil {
		log.Warningf("Buffer copying ended with an error. Closing the proxy. Cause: %s", err)
	} else if len == 0 {
		log.Warningf("Buffer copying ended after transfering zero bytes. One of the connections was likely corrupt. Closing the proxy. Cause is uknown (len == 0)")
	}
	return err
}
