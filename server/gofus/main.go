package main

import (
	"fmt"
	"net/http"
)

func main() {
	hub := NewHub()
	connGateway := NewConnGateway()

	listenConns := func() {
		for {
			select {
			case peer := <-connGateway.NewPeer:
				go hub.Negotiate(peer)
			}
		}
	}

	go listenConns()

	http.HandleFunc("/signal", connGateway.HandleNewPeerConnection)

	fmt.Printf("GOFUS\nListening on 3000...\n\n")
	http.ListenAndServe(":3000", nil)
}
