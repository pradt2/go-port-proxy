package server

import (
	"net"
	"math/rand"
	"project-proxy/messaging"
	"project-proxy/logs"
	"encoding/binary"
	"project-proxy/connectivity"
	"time"
)

type server struct {
	messenger           messaging.MessengerOverlay
	remoteConnFactory   connectivity.ConnFactory
	transferConnFactory connectivity.ConnFactory
	pingInterval        time.Duration
	bufferSize          uint64
	localConns          map[uint32]net.Conn
	waitUntilFinished   chan bool
}

type Config struct {
}

type Server interface {
	Start()
	Wait()
}

var log = logs.GetLoggerForModule("server")

func NewServer(remoteConnFactory connectivity.ConnFactory, transferConnFactory connectivity.ConnFactory, pingInterval time.Duration, bufferSize uint64, overlay messaging.MessengerOverlay) Server {
	return &server{
		messenger:           overlay,
		remoteConnFactory:   remoteConnFactory,
		transferConnFactory: transferConnFactory,
		pingInterval:        pingInterval,
		bufferSize:          bufferSize,
		localConns:          make(map[uint32]net.Conn),
		waitUntilFinished:   make(chan bool),
	}
}

func (s *server) Start() {
	onReceive := func(id uint32, service uint32, payload []byte, err error) {
		if err != nil {
			log.Warningf("Received an errorous message - id: %d , service: %d This message will be ignored. Cause: %s", id, service, err)
			return
		}
		log.Debugf("Received a forward message - id: %d , service: %d, len: %d", id, service, len(payload))
		conn := s.localConns[id]
		if conn == nil {
			log.Warningf("Attempting to write to a non-existent remote connection. This message will be ignored. Sending request to close local connection")
			s.messenger.SendCloseConn(id)
			return
		}
		log.Debugf("Writing bytes (total: %d) to remote connection id: %d", len(payload), id)
		length, err := conn.Write(payload)
		if err != nil {
			log.Errorf("Error while writing to remote connection. Closing local connection. Sending request to close local connection. Cause: %s", err)
			conn.Close()
			delete(s.localConns, id)
			s.messenger.SendCloseConn(id)
			return
		} else if length == 0 {
			log.Warningf("Written no bytes to remote connection id: %d This usually indicates a timeout. Closing remote connection. Sending request to close local connection", id)
			conn.Close()
			delete(s.localConns, id)
			s.messenger.SendCloseConn(id)
			return
		}
		log.Debugf("Successfully written %d of %d bytes to a remote connection id: %d", length, len(payload), id)
	}
	onCloseConn := func(remoteConnId uint32, err error) {
		if err != nil {
			log.Errorf("Erroreous request to close a remote connection. This message will be ignored. Cause: %s", err)
			return
		}
		log.Infof("Received a request to close a remote connection id: %d", remoteConnId)
		conn := s.localConns[remoteConnId]
		if conn == nil {
			log.Warningf("Cannot close remote connection id: %d Unknown connection. This message will be ignored", remoteConnId)
			return
		}
		log.Infof("Closing remote connection connection id: %d", remoteConnId)
		error := conn.Close()
		delete(s.localConns, remoteConnId)
		if error != nil {
			log.Warningf("Closing a remote connection id: %d failed. Connection was removed from the connection list. Cause: %s", error)
			return
		}
		log.Infof("Successfuly closed remote connection id: %d", remoteConnId)
	}
	var remoteListener net.Listener
	var transferListener net.Listener
	onControlConnLost := func(err error) {
		log.Errorf("Control connection lost. Closing all remote connections. Stopping listening for new remote connections. Signalling that the server has finished. Cause: %s", err)
		remoteListener.Close()
		transferListener.Close()
		for _, v := range s.localConns {
			v.Close()
		}
		s.waitUntilFinished <- true
	}
	s.messenger.SetOnForwardListener(onReceive)
	s.messenger.SetOnCloseConnectionListener(onCloseConn)
	s.messenger.SetOnControlConnectionLostListener(onControlConnLost)

	log.Infof("Starting server - network type: %s, remote connections address: %s", s.remoteConnFactory.GetNetworkType(), s.remoteConnFactory.GetAddress())
	s.messenger.Start()
	if s.pingInterval > 0 {
		log.Infof("The server will be pinged every %d ms", s.pingInterval/time.Millisecond)
		s.startKeepAlive()
	} else {
		log.Warningf("The agent will not be pinged (setting value: %d). It can cause timeout problems across NATs or filewalls", s.pingInterval)
	}

	log.Infof("Trying to listen for remote connections - network type: %s, address: %s", s.remoteConnFactory.GetNetworkType(), s.remoteConnFactory.GetAddress())
	remoteListener, err := s.remoteConnFactory.Listen()
	if err != nil {
		log.Fatalf("Could not listen for remote connections. Is the addr: %s used already? Cause: %s", s.remoteConnFactory.GetAddress(), err)
		return
	}
	transferListener, err = s.transferConnFactory.Listen()
	if err != nil {
		log.Fatalf("Could not listen for transfer connections. Is the addr: %s used already? Cause: %s", "0.0.0.0:8888", err)
		return
	}

	log.Infof("Listening for remote connections")

	go func() {
		for {
			conn, err := remoteListener.Accept()
			if err != nil {
				log.Errorf("Error while accepting remote connection. The listening has likely stopped. No more remote connections will be accepted. Cause: %s", err)
				s.waitUntilFinished <- true
				return
			}
			randId := rand.Uint32()
			log.Infof("Accepted a new remote connection, assigning id: %d", randId)
			s.localConns[randId] = conn
			s.messenger.SendOpenConn(randId, 0)
		}
	}()

	go func() {
		for {
			transferConn, err := transferListener.Accept()
			if err != nil {
				log.Errorf("Error while accepting transfer connection. The listening has likely stopped. No more transfer connections will be accepted. Cause: %s", err)
				s.waitUntilFinished <- true
				return
			}
			connId := uint32(0)
			err = binary.Read(transferConn, binary.LittleEndian, &connId)
			if err != nil {
				log.Fatalf("Decode failed: %s", err)
			}
			remoteConn := s.localConns[connId]
			if remoteConn == nil {
				log.Errorf("Proxy cannot be created. Closing the transfer connection. Cause: could not find remote connection id: %d", connId)
				transferConn.Close()
			}
			connProxy := connectivity.NewConnProxy(remoteConn, transferConn)
			connProxy.SetOnFinishedListener(func(connA net.Conn, connB net.Conn) {
				delete(s.localConns, connId)
			})
			connProxy.RunAsync()
		}
	}()
}

func (s *server) Wait() {
	<-s.waitUntilFinished
	log.Info("The server has finished")
}

func (s *server) startKeepAlive() {
	go func() {
		for {
			log.Info("Sending ping to the agent")
			err := s.messenger.SendPing()
			if err != nil {
				log.Warningf("Pinging resulted in an error. The server is likely going to restart. Stopping pinging. Cause: %s", err)
				break
			}
			time.Sleep(s.pingInterval)
		}
	}()
}
