package main

import (
	"fmt"
	// "github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

type Room struct {
	name                 string
	broadcasterPeerConn *webrtc.PeerConnection
}

func main() {
	fmt.Println("Hello, world!")
}