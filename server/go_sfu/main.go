package main

import (
	"fmt"
	"net/http"
	// "github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	"github.com/gorilla/websocket"
)

type Peer struct {
	name string
	conn *webrtc.PeerConnection
}

type Room struct {
	name        string
	broadcaster Peer
	watchers    []Peer
}

var upgrader = websocket.Upgrader{}

func echo(res http.ResponseWriter, req *http.Request) {

}

func main() {
	fmt.Println("Hello, world!")
}