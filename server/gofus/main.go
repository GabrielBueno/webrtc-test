package main

import (

	// "github.com/pion/rtcp"

	"fmt"
	"net/http"

	"github.com/pion/webrtc/v3"
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

type Hub struct {
	rooms map[string]Room
}

func main() {
	pool := NewSignallingConnectionPool()

	listenConns := func() {
		for {
			select {
			case conn := <-pool.NewConnection:
				go Negotiate(conn)
			}
		}
	}

	go listenConns()

	http.HandleFunc("/signal", pool.CreateSignallingConnection)

	fmt.Printf("GOFUS\nListening on 8083...\n\n")
	http.ListenAndServe(":8083", nil)
}
