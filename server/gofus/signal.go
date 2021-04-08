package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

const (
	BROADCAST_INTENTION = "broadcast"
	WATCH_INTENTION     = "watch"
	FINISH_INTENTION    = "finish"
	DENY_INTENTION      = "deny"
)

type SignalMessage struct {
	Intention string
	Detail    string
	Sdp       *webrtc.SessionDescription
}

func (msg SignalMessage) IsClosingMessage() bool {
	return msg.Intention == FINISH_INTENTION || msg.Intention == DENY_INTENTION
}

type SignallingConnection struct {
	WsConn   *websocket.Conn
	Incoming chan SignalMessage
	Sending  chan SignalMessage
	Closing  chan bool
	IsOpen   bool
}

func (signalling *SignallingConnection) Send(msg SignalMessage) {
	if signalling.IsOpen {
		signalling.Sending <- msg
	}
}

func (signalling *SignallingConnection) listenIncomingMessages() {
	for signalling.IsOpen {
		_, bytes, err := signalling.WsConn.ReadMessage()

		if !signalling.IsOpen {
			break
		}

		if err != nil {
			log.Printf("err: on message receive: %s\n", err)
			break
		}

		log.Println("received sucessfuly")

		var signalMessage SignalMessage
		err = json.Unmarshal(bytes, &signalMessage)

		if err != nil {
			log.Printf("err: while unmarshling incoming json: %s\n", err)
			break
		}

		log.Printf("sending %v", signalMessage)
		signalling.Incoming <- signalMessage
	}
}

func (signalling *SignallingConnection) listenSendingMessages() {
	for signalling.IsOpen {
		select {
		case msg := <-signalling.Sending:
			err := signalling.WsConn.WriteJSON(msg)

			if err != nil {
				log.Printf("err: while sending message: %s\n", err)
			}

			if msg.IsClosingMessage() {
				signalling.Close()
			}
		}
	}
}

func (signalling *SignallingConnection) Close() {
	signalling.WsConn.Close()

	log.Println("Closing incoming chan")
	close(signalling.Incoming)

	log.Println("Closing incoming send")
	close(signalling.Sending)

	signalling.IsOpen = false
	signalling.Closing <- true
}

//
type SignallingConectionPool struct {
	NewConnection chan *SignallingConnection
}

func NewSignallingConnectionPool() SignallingConectionPool {
	return SignallingConectionPool{NewConnection: make(chan *SignallingConnection)}
}

func (pool *SignallingConectionPool) CreateSignallingConnection(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Printf("e: couldn't estabilish a websocket connection: %s", err)
		return
	}

	log.Printf("received connection from %s\n", r.Header.Get("Origin"))

	signalling := SignallingConnection{
		WsConn:   conn,
		Incoming: make(chan SignalMessage),
		Sending:  make(chan SignalMessage),
		IsOpen:   true,
	}

	go signalling.listenIncomingMessages()
	go signalling.listenSendingMessages()

	pool.NewConnection <- &signalling
}

func (pool *SignallingConectionPool) Close() {
	close(pool.NewConnection)
}
