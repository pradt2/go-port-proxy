package messaging

import (
	"net"
	"encoding/gob"
	"time"
	"project-proxy/logs"
)

type messenger struct {
	conn                 net.Conn
	timeout              time.Duration
	enc                  *gob.Encoder
	dec                  *gob.Decoder
	stopListeningOnErr   bool
	onMessageReceived    func(message interface{}, err error)
	getNewMessagePointer func() interface{}
}

type Messenger interface {
	SetOnMessageReceived(onReceived func(message interface{}, err error), getNewStructPointer func() interface{})
	SetTimeout(timeout time.Duration)
	Send(message interface{}) error
	Start()
}

var l = logs.GetLoggerForModule("mess")

func NewMessenger(conn net.Conn) Messenger {
	return &messenger{
		conn:                 conn,
		enc:                  gob.NewEncoder(conn),
		dec:                  gob.NewDecoder(conn),
		stopListeningOnErr:   true,
		onMessageReceived:    nil,
		getNewMessagePointer: nil,
	}
}

func (m *messenger) SetOnMessageReceived(onMessageReceived func(message interface{}, err error), getNewStructPointer func() interface{}) {
	m.onMessageReceived = onMessageReceived
	m.getNewMessagePointer = getNewStructPointer
}

func (m *messenger) SetTimeout(timeout time.Duration) {
	m.timeout = timeout
}

func (m *messenger) Send(message interface{}) error {
	if m.timeout != 0 {
		m.conn.SetWriteDeadline(time.Now().Add(m.timeout))
	}
	err := m.enc.Encode(message)
	if err != nil {
		l.Errorf("Could not encode a message. Closing the connection. Cause: %s", err)
		m.conn.Close()
	}
	return err
}

func (m *messenger) Start() {
	if m.timeout == 0 {
		l.Warningf("Timeout is disabled. Please check if this is as intended")
	}
	go func() {
		for {
			message := m.getNewMessagePointer()
			if m.timeout != 0 {
				m.conn.SetReadDeadline(time.Now().Add(m.timeout))
			}
			err := m.dec.Decode(message)
			if err != nil {
				l.Errorf("Could not decode a message. Cause: %s", err)
				m.onMessageReceived(nil, err)
				if m.stopListeningOnErr {
					log.Info("Closing the control connection since [config] stopListeningOnErr is TRUE")
					m.conn.Close()
					return
				}
			}
			if m.onMessageReceived != nil {
				m.onMessageReceived(message, nil)
			}
		}
	}()
}
