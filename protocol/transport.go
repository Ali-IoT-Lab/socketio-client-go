package protocol

import (
	"bytes"
	"errors"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
)

type Conn interface {
	io.Closer
	Read() (*Packet, error)
	Write(*Packet) error
}

type Transport interface {
	Dial(string, http.Header) (Conn, error)
}

type webSocketConn struct {
	conn *websocket.Conn
}

func (conn *webSocketConn) Close() error {
	return conn.conn.Close()
}

func (conn *webSocketConn) Read() (*Packet, error) {
	msgType, r, err := conn.conn.NextReader()
	if err != nil {
		return nil, err
	}
	if msgType == websocket.BinaryMessage {
		return nil, errors.New("binary messages are not supported")
	}
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	if err != nil {
		return nil, err
	}
	return decodePacket(buf.String())
}

func (conn *webSocketConn) Write(packet *Packet) error {
	w, err := conn.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	if data, err := packet.Encode(); err != nil {
		return err
	} else if _, err = w.Write(data); err != nil {
		return err
	} else if err = w.Close(); err != nil {
		return err
	}
	return nil
}

type webSocketTransport struct {
}

func NewWebSocketTransport() Transport {
	return &webSocketTransport{}
}

func (t *webSocketTransport) Dial(url string, requestHeader http.Header) (Conn, error) {
	dialer := &websocket.Dialer{}
	conn, _, err := dialer.Dial(url, requestHeader)
	if err != nil {
		return nil, err
	}
	return &webSocketConn{conn}, nil
}
