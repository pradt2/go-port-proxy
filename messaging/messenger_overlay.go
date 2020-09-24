package messaging

import (
	"project-proxy/logs"
)

const (
	Forward         uint8 = iota
	Ping
	OpenConnection
	CloseConnection
)

type message struct {
	Type         uint8
	RemoteConnId uint32
	Service      uint32
	Payload      []byte
}

type messengerOverlay struct {
	messenger         Messenger
	onForward         func(remoteConnId uint32, service uint32, payload []byte, err error)
	onOpenConn        func(remoteConnId uint32, service uint32, err error)
	onCloseConn       func(remoteConnId uint32, err error)
	onControlConnLost func(err error)
}

type MessengerOverlay interface {
	Start()
	SetOnForwardListener(onForward func(remoteConnId uint32, service uint32, payload []byte, err error))
	SetOnOpenConnectionListener(onOpenConn func(remoteConnId uint32, service uint32, err error))
	SetOnCloseConnectionListener(onCloseConn func(remoteConnId uint32, err error))
	SetOnControlConnectionLostListener(onControlConnLost func(err error))
	SendForward(remoteConnId uint32, service uint32, payload []byte) error
	SendOpenConn(remoteConnId uint32, service uint32) error
	SendCloseConn(remoteConnId uint32) error
	SendPing() error
}

var log = logs.GetLoggerForModule("mess_ovr")

func NewMessengerOverlay(messenger Messenger) MessengerOverlay {
	return &messengerOverlay{
		messenger:         messenger,
		onForward:         nil,
		onOpenConn:        nil,
		onCloseConn:       nil,
		onControlConnLost: nil,
	}
}

func (m *messengerOverlay) Start() {
	m.messenger.SetOnMessageReceived(func(msg interface{}, err error) {
		if err != nil {
			log.Errorf("Control connection has failed to receive a message. Executing onControlConnLost. Cause: %s", err)
			m.onControlConnLost(err)
			return
		}
		parsedMessage, ok := msg.(*message)
		if ok != true {
			log.Errorf("Control connection message could not be parsed. Executing onControlConnLost. Cause is unknown")
			m.onControlConnLost(nil)
		}
		switch parsedMessage.Type {
		case Forward:
			m.onForward(parsedMessage.RemoteConnId, parsedMessage.Service, parsedMessage.Payload, err)
		case OpenConnection:
			m.onOpenConn(parsedMessage.RemoteConnId, parsedMessage.Service, err)
		case CloseConnection:
			m.onCloseConn(parsedMessage.RemoteConnId, err)
		case Ping:
			log.Info("Ping message has been received")
		}
	}, func() interface{} {
		return &message{}
	})
	m.messenger.Start()
}

func (m *messengerOverlay) SetOnForwardListener(onForward func(remoteConnId uint32, service uint32, payload []byte, err error)) {
	m.onForward = onForward
}

func (m *messengerOverlay) SetOnOpenConnectionListener(onOpenConn func(remoteConnId uint32, service uint32, err error)) {
	m.onOpenConn = onOpenConn
}

func (m *messengerOverlay) SetOnCloseConnectionListener(onCloseConn func(remoteConnId uint32, err error)) {
	m.onCloseConn = onCloseConn
}

func (m *messengerOverlay) SetOnControlConnectionLostListener(onControlConnLost func(err error)) {
	m.onControlConnLost = onControlConnLost
}

func (m *messengerOverlay) SendForward(remoteConnId uint32, service uint32, payload []byte) error {
	err := m.messenger.Send(&message{
		Type:         Forward,
		RemoteConnId: remoteConnId,
		Service:      service,
		Payload:      payload,
	})
	if err != nil {
		log.Errorf("Could not send a forward message. Executing onControlConnLost. Cause: %s", err)
		m.onControlConnLost(err)
	}
	return err
}

func (m *messengerOverlay) SendOpenConn(remoteConnId uint32, service uint32) error {
	err := m.messenger.Send(&message{
		Type:         OpenConnection,
		RemoteConnId: remoteConnId,
		Service:      service,
	})
	if err != nil {
		log.Errorf("Could not send a open conn message. Executing onControlConnLost. Cause: %s", err)
		m.onControlConnLost(err)
	}
	return err
}

func (m *messengerOverlay) SendCloseConn(remoteConnId uint32) error {
	err := m.messenger.Send(&message{
		Type:         CloseConnection,
		RemoteConnId: remoteConnId,
	})
	if err != nil {
		log.Errorf("Could not send a close conn message. Executing onControlConnLost. Cause: %s", err)
		m.onControlConnLost(err)
	}
	return err
}

func (m *messengerOverlay) SendPing() error {
	err := m.messenger.Send(&message{
		Type: Ping,
	})
	if err != nil {
		log.Errorf("Could not send a ping message. Executing onControlConnLost. Cause: %s", err)
		m.onControlConnLost(err)
	}
	return err
}
