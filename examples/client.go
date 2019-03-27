package main

import (
	"fmt"
	"net/http"
	"time"
	"github.com/Ali-IoT-Lab/socketio-client-go"
)

func main() {
	var Header http.Header = map[string][]string{
		"moja":     {"ccccc, asdasdasdasd"},
		"terminal": {"en-esadasdasdwrw"},
		"success":  {"dasdadas", "wdsadaderew"},
	}
	fmt.Println("-------------------request.header-------------------------")
	fmt.Println(Header)
	s, err := socketio.Socket("ws://127.0.0.1:3000")
	if err != nil {
		panic(err)
	}
	s.Connect(Header)
	s.On("message", func(args ...interface{}) {
		fmt.Println("servver message!")
		fmt.Println(args[0])
	})
	for {
		s.Emit("messgae", "hello server!")
		time.Sleep(2 * time.Second)
	}
}
