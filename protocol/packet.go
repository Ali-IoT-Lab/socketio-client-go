package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type PacketType int

// https://github.com/socketio/engine.io-protocol#packet
const (
	PacketTypeOpen PacketType = iota
	PacketTypeClose
	PacketTypePing
	PacketTypePong
	PacketTypeMessage
	PacketTypeUpgrade
	PacketTypeNoop
)

func (t PacketType) String() string {
	return strconv.Itoa(int(t))
}

type Packet struct {
	Type    PacketType
	payload string
}

func decodePacket(payload string) (*Packet, error) {
	if len(payload) == 0 {
		return nil, errors.New("invalid payload")
	}
	i, err := strconv.Atoi(payload[0:1])
	if err != nil {
		return nil, err
	}
	typ := PacketType(i)
	if typ < PacketTypeOpen || typ > PacketTypeNoop {
		return nil, errors.New("invalid packet type")
	}
	return &Packet{typ, payload[1:]}, nil
}

func (p *Packet) String() string {
	return fmt.Sprintf("type: %s\npayload: %s", p.Type, p.payload)
}

func NewPingPacket() *Packet {
	return &Packet{Type: PacketTypePing}
}

func NewPongPacket() *Packet {
	return &Packet{Type: PacketTypePong}
}

func (p *Packet) DecodeHandshake() (*Handshake, error) {
	if p.Type != PacketTypeOpen {
		return nil, errors.New("packet type is not PacketTypeOpen")
	}
	var h Handshake
	err := json.Unmarshal([]byte(p.payload), &h)
	return &h, err
}

func (p *Packet) DecodeMessage() (*Message, error) {
	if p.Type != PacketTypeMessage {
		return nil, errors.New("packet type is not PacketTypeMessage")
	}
	payload := p.payload
	if len(payload) == 0 {
		return nil, errors.New("invalid packet payload")
	}
	i, err := strconv.Atoi(payload[0:1])
	if err != nil {
		return nil, err
	}
	typ := MessageType(i)
	if typ < MessageTypeConnect || typ > MessageTypeError {
		return nil, errors.New("invalid packet message type")
	}
	m := &Message{
		Type:      typ,
		Namespace: "/",
		ID:        -1,
	}
	// extract namespace info
	payload = payload[1:]
	if len(payload) > 0 && strings.HasPrefix(payload, "/") {
		var b strings.Builder
		var i int
		for _, ch := range payload {
			i++
			if ch == ',' {
				break
			}
			b.WriteRune(ch)
		}
		m.Namespace = b.String()
		payload = payload[i:]
	}
	// extract id info
	if len(payload) > 0 {
		var b strings.Builder
		var i int
		for _, ch := range payload {
			i++
			if !unicode.IsDigit(ch) {
				i--
				break
			}
			b.WriteRune(ch)
		}
		if i > 0 {
			m.ID, _ = strconv.Atoi(b.String())
			payload = payload[i:]
		}
	}
	// encode message payloads
	switch m.Type {
	case MessageTypeAck:
		var payloads []interface{}
		if err = json.Unmarshal([]byte(payload), &payloads); err != nil {
			return nil, err
		}
		m.Payloads = payloads
	case MessageTypeEvent:
		var payloads []interface{}
		if err = json.Unmarshal([]byte(payload), &payloads); err != nil {
			return nil, err
		}
		if len(payloads) == 0 {
			return nil, errors.New("empty event payload")
		}
		event, ok := payloads[0].(string)
		if !ok {
			return nil, errors.New("invalid event payload: " + payload)
		}
		m.Event = event
		m.Payloads = payloads[1:]
	case MessageTypeError:
		m.Payloads = []interface{}{payload}
	}
	return m, nil
}

func (p *Packet) Encode() ([]byte, error) {
	if p.Type < PacketTypeOpen || p.Type > PacketTypeNoop {
		return nil, errors.New("invalid packet type")
	}
	return []byte(p.Type.String() + p.payload), nil
}

type MessageType int

// https://github.com/socketio/socket.io-protocol#parsertypes
const (
	MessageTypeConnect MessageType = iota
	MessageTypeDisconnect
	MessageTypeEvent
	MessageTypeAck
	MessageTypeError
	MessageTypeBinaryEvent
	MessageTypeBinaryAck
)

func (t MessageType) String() string {
	return strconv.Itoa(int(t))
}

type Message struct {
	Type      MessageType
	Namespace string
	ID        int
	Event     string
	Payloads  []interface{}
}

func (m *Message) Encode() (*Packet, error) {
	var b strings.Builder
	b.WriteString(m.Type.String())
	if m.Namespace != "" && m.Namespace != "/" {
		b.WriteString(m.Namespace)
		if m.ID >= 0 || len(m.Payloads) > 0 {
			b.WriteRune(',')
		}
	}
	if m.ID >= 0 {
		b.WriteString(strconv.Itoa(m.ID))
	}
	payload := make([]interface{}, 0)
	if m.Event != "" {
		payload = append(payload, m.Event)
	}
	if len(m.Payloads) > 0 {
		payload = append(payload, m.Payloads...)
	}
	if len(payload) > 0 {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		b.Write(data)
	}
	return &Packet{PacketTypeMessage, b.String()}, nil
}

type Handshake struct {
	Sid          string   `json:"sid"`
	Upgrades     []string `json:"upgrades"`
	PingInterval int      `json:"pingInterval"`
	PingTimeout  int      `json:"pingTimeout"`
}
