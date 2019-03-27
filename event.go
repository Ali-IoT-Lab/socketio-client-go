package socketio

import (
	"sync"
)

const (
	EventOpen      string = "open"
	EventConnect          = "connect"
	EventReconnect        = "reconnect"
	EventError            = "error"
)

type Listener func(args ...interface{})

type emitter struct {
	listeners map[string][]Listener
	m         sync.RWMutex
}

func (e *emitter) On(event string, listener Listener) {
	e.m.Lock()
	defer e.m.Unlock()
	listeners, ok := e.listeners[event]
	if ok {
		listeners = append(listeners, listener)
	} else {
		listeners = []Listener{listener}
	}
	e.listeners[event] = listeners
}

func (e *emitter) emit(event string, args ...interface{}) bool {
	e.m.RLock()
	listeners, ok := e.listeners[event]
	if ok {
		for _, listener := range listeners {
			listener(args...)
		}
	}
	e.m.RUnlock()
	return ok
}
