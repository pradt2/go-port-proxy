package agent

import (
	"net"
	"project-proxy/messaging"
	"time"
	"project-proxy/logs"
	"encoding/binary"
	"project-proxy/connectivity"
)

type agent struct {
	messenger           messaging.MessengerOverlay
	localConnFactory    connectivity.ConnFactory
	transferConnFactory connectivity.ConnFactory
	pingInterval        time.Duration
	bufferSize          uint64
	localConns          map[uint32]net.Conn
	waitUntilFinished   chan bool
}

type Agent interface {
	Start()
	Wait()
}

var log = logs.GetLoggerForModule("agent")

func NewAgent(localConnFactory connectivity.ConnFactory, tranferConnFactory connectivity.ConnFactory, pingInterval time.Duration, bufferSize uint64, overlay messaging.MessengerOverlay) Agent {
	return &agent{
		messenger:           overlay,
		localConnFactory:    localConnFactory,
		transferConnFactory: tranferConnFactory,
		pingInterval:        pingInterval,
		bufferSize:          bufferSize,
		localConns:          make(map[uint32]net.Conn),
		waitUntilFinished:   make(chan bool),
	}
}

func (a *agent) Start() {
	onReceive := func(id uint32, service uint32, payload []byte, err error) () {
		if err != nil {
			log.Warningf("Received an erroreous message - id: %d , service: %d This message will be ignored. Cause: %s", id, service, err)
			return
		}
		log.Debugf("Received a forward message - id: %d , service: %d, len: %d", id, service, len(payload))
		localConn := a.localConns[id]
		if localConn == nil {
			log.Infof("Connection with id: %d not found. Opening new local connection", id)
			localConn, err = a.localConnFactory.Connect()
			if err != nil {
				log.Errorf("Error while opening new local connection. Sending request to close remote connection. Cause: %s", err)
				a.messenger.SendCloseConn(id)
			}
			a.localConns[id] = localConn
			go func() {
				log.Infof("Creating a new buffer of %d bytes for a new local connection id: %d", a.bufferSize, id)
				buffer := make([]byte, a.bufferSize)
				for {
					len, err := localConn.Read(buffer)
					log.Debugf("Read %d bytes from local connection id: %d", len, id)
					if err != nil {
						log.Errorf("Error while reading local connection id: %d Closing local connection. Sending request to close remote connection. Cause: %s", id, err)
						localConn.Close()
						delete(a.localConns, id)
						a.messenger.SendCloseConn(id)
						return
					} else if len == 0 {
						log.Warningf("Read no bytes from local connection id: %d This usually indicates a timeout. Closing local connection. Sending request to close remote connection", id)
						localConn.Close()
						delete(a.localConns, id)
						a.messenger.SendCloseConn(id)
						return
					}
					log.Debugf("Sending %d bytes to the server", len)
					a.messenger.SendForward(id, service, buffer[:len])
				}
			}()
		}
		log.Debugf("Writing bytes (total: %d) to local connection id: %d", len(payload), id)
		length, err := localConn.Write(payload)
		if err != nil {
			log.Errorf("Error while writing to local connection. Closing local connection. Sending request to close remote connection. Cause: %s", err)
			localConn.Close()
			delete(a.localConns, id)
			a.messenger.SendCloseConn(id)
			return
		} else if length == 0 {
			log.Warningf("Written no bytes to local connection id: %d This usually indicates a timeout. Closing local connection. Sending request to close remote connection", id)
			localConn.Close()
			delete(a.localConns, id)
			a.messenger.SendCloseConn(id)
			return
		}
		log.Debugf("Successfully written %d of %d bytes to a local connection id: %d", length, len(payload), id)
	}
	onOpenConn := func(remoteConnId uint32, service uint32, err error) {
		if err != nil {
			log.Errorf("Erroreous request to open a local connection. This message will be ignored. Cause: %s", err)
			return
		}
		transferConn, err := a.transferConnFactory.Connect()
		if err != nil {
			log.Errorf("Transfer connection could not be established. This message will be ignored. Cause: %s", err)
			return
		}
		binary.Write(transferConn, binary.LittleEndian, remoteConnId)

		localConn, err := a.localConnFactory.Connect()
		if err != nil {
			log.Errorf("Error while opening new local connection. Closing transfer connection. Cause: %s", err)
			transferConn.Close()
		}
		p := connectivity.NewConnProxy(transferConn, localConn)
		p.RunAsync()
	}
	onCloseConn := func(remoteConnId uint32, err error) {
		if err != nil {
			log.Errorf("Erroreous request to close a local connection. This message will be ignored. Cause: %s", err)
			return
		}
		log.Infof("Received a request to close a local connection id: %d", remoteConnId)
		conn := a.localConns[remoteConnId]
		if conn == nil {
			log.Warningf("Cannot close local connection id: %d Unknown connection. This message will be ignored", remoteConnId)
			return
		}
		log.Infof("Closing local connection connection id: %d", remoteConnId)
		error := conn.Close()
		delete(a.localConns, remoteConnId)
		if error != nil {
			log.Warningf("Closing a local connection id: %d failed. Connection was removed from the connection list. Cause: %s", error)
			return
		}
		log.Infof("Successfuly closed local connection id: %d", remoteConnId)
	}
	onControlConnLost := func(err error) {
		log.Errorf("Control connection lost. Closing all local connections. Signalling that the agent has finished. Cause: %s", err)
		for _, v := range a.localConns {
			v.Close()
		}
		a.waitUntilFinished <- true
	}
	a.messenger.SetOnForwardListener(onReceive)
	a.messenger.SetOnOpenConnectionListener(onOpenConn)
	a.messenger.SetOnCloseConnectionListener(onCloseConn)
	a.messenger.SetOnControlConnectionLostListener(onControlConnLost)

	log.Infof("Starting agent - network type: %s, local connections address: %s, control connection ping interval: %d", a.localConnFactory.GetNetworkType(), a.localConnFactory.GetAddress(), a.pingInterval)
	a.messenger.Start()
	if a.pingInterval > 0 {
		log.Infof("The server will be pinged every %d ms", a.pingInterval/time.Millisecond)
		a.startKeepAlive()
	} else {
		log.Warningf("The server will not be pinged (setting value: %d). It can cause timeout problems across NATs or filewalls", a.pingInterval)
	}
}

func (a *agent) Wait() {
	<-a.waitUntilFinished
	log.Infof("The agent has finished")
}

func (a *agent) startKeepAlive() {
	go func() {
		for {
			log.Info("Sending ping to the server")
			err := a.messenger.SendPing()
			if err != nil {
				log.Warningf("Pinging resulted in an error. The agent is likely going to restart. Stopping pinging. Cause: %s", err)
				break
			}
			time.Sleep(a.pingInterval)
		}
	}()
}
