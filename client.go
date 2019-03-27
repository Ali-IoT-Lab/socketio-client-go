package socketio

import (
	"math"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/Ali-IoT-Lab/socketio-client-go/protocol"
)

const (
	stateOpen uint32 = iota
	stateConnecting
	stateReady
	stateReconnecting
	stateClose
)

type option struct {
	AutoReconnect    bool
	MaxReconnections int32
}

var defaultOption = &option{
	AutoReconnect:    true,
	MaxReconnections: math.MaxInt32,
}

type socketClient struct {
	emitter
	state     uint32
	url       *url.URL
	option    *option
	transprot protocol.Transport
	outChan   chan *protocol.Packet
	closeChan chan bool
}

func Socket(urlstring string) (*socketClient, error) {
	u, err := url.Parse(urlstring)
	if err != nil {
		return nil, err
	}
	u.Path = "/socket.io/"
	q := u.Query()
	q.Add("EIO", "3")
	q.Add("transport", "websocket")
	u.RawQuery = q.Encode()
	return &socketClient{
		emitter:   emitter{listeners: make(map[string][]Listener)},
		url:       u,
		option:    defaultOption,
		transprot: protocol.NewWebSocketTransport(),
		outChan:   make(chan *protocol.Packet, 64),
		closeChan: make(chan bool),
	}, nil
}

func (s *socketClient) Connect(requestHeader http.Header) {
	if atomic.CompareAndSwapUint32(&s.state, stateOpen, stateConnecting) {
		conn, err := s.transprot.Dial(s.url.String(), requestHeader)
		if err != nil {
			s.emit(EventError, err)
			go s.reconnect(stateConnecting, requestHeader)
			return
		}
		if atomic.CompareAndSwapUint32(&s.state, stateConnecting, stateReady) {
			go s.start(conn, requestHeader)
			s.emit(EventConnect)
		} else {
			conn.Close()
		}
	}
}

func (s *socketClient) Disconnect() {
	atomic.StoreUint32(&s.state, stateClose)
	close(s.outChan)
	close(s.closeChan)
}

func (s *socketClient) Emit(event string, args ...interface{}) {
	if atomic.LoadUint32(&s.state) == stateReady && !s.emit(event, args) {
		m := &protocol.Message{
			Type:      protocol.MessageTypeEvent,
			Namespace: "/",
			ID:        -1,
			Event:     event,
			Payloads:  args,
		}
		p, err := m.Encode()
		if err != nil {
			s.emit(EventError, err)
		} else {
			s.outChan <- p
		}
	}
}

func (s *socketClient) reconnect(state uint32, requestHeader http.Header) {
	time.Sleep(time.Second)
	if atomic.CompareAndSwapUint32(&s.state, state, stateReconnecting) {
		conn, err := s.transprot.Dial(s.url.String(), requestHeader)
		if err != nil {
			s.emit(EventError, err)
			go s.reconnect(stateReconnecting, requestHeader)
			return
		}
		if atomic.CompareAndSwapUint32(&s.state, stateReconnecting, stateReady) {
			go s.start(conn, requestHeader)
			s.emit(EventReconnect)
		} else {
			conn.Close()
		}
	}
}

func (s *socketClient) start(conn protocol.Conn, requestHeader http.Header) {
	stopper := make(chan bool)
	go s.startRead(conn, stopper)
	go s.startWrite(conn, stopper)
	select {
	case <-stopper:
		go s.reconnect(stateReady, requestHeader)
		conn.Close()
	case <-s.closeChan:
		conn.Close()
	}
}

func (s *socketClient) startRead(conn protocol.Conn, stopper chan bool) {
	defer func() {
		recover()
	}()
	for atomic.LoadUint32(&s.state) == stateReady {
		p, err := conn.Read()
		if err != nil {
			s.emit(EventError, err)
			close(stopper)
			return
		}
		switch p.Type {
		case protocol.PacketTypeOpen:
			h, err := p.DecodeHandshake()
			if err != nil {
				s.emit(EventError, err)
			} else {
				go s.startPing(h, stopper)
			}
		case protocol.PacketTypePing:
			s.outChan <- protocol.NewPongPacket()
		case protocol.PacketTypeMessage:
			m, err := p.DecodeMessage()
			if err != nil {
				s.emit(EventError, err)
			} else {
				s.emit(m.Event, m.Payloads...)
			}
		}
	}
}

func (s *socketClient) startWrite(conn protocol.Conn, stopper chan bool) {
	defer func() {
		recover()
	}()
	for atomic.LoadUint32(&s.state) == stateReady {
		select {
		case <-stopper:
			return
		case p, ok := <-s.outChan:
			if !ok {
				return
			}
			err := conn.Write(p)
			if err != nil {
				s.emit(EventError, err)
				close(stopper)
				return
			}
		}

	}
}

func (s *socketClient) startPing(h *protocol.Handshake, stopper chan bool) {
	defer func() {
		recover()
	}()
	for {
		time.Sleep(time.Duration(h.PingInterval) * time.Millisecond)
		select {
		case <-stopper:
			return
		case <-s.closeChan:
			return
		default:
		}
		if atomic.LoadUint32(&s.state) != stateReady {
			return
		}
		s.outChan <- protocol.NewPingPacket()
	}
}
