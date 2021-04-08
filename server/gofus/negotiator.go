package main

import (
	"log"
)

func processMessage(conn *SignallingConnection, msg SignalMessage) {
	log.Printf("Proccessing: %v\n", msg)

	switch msg.Intention {
	case BROADCAST_INTENTION:
		log.Println("wants to broadcast")

	case WATCH_INTENTION:
		log.Println("wants to watch")

	case FINISH_INTENTION:
		conn.Sending <- SignalMessage{Intention: FINISH_INTENTION}

	default:
		log.Printf("err: invalid intention for negotiation: %s\n", msg.Intention)

		denialMsg := SignalMessage{
			Intention: DENY_INTENTION,
			Detail:    msg.Intention + " is an invalid intention",
		}

		conn.Send(denialMsg)
	}
}

func Negotiate(conn *SignallingConnection) {
	for conn.IsOpen {
		select {
		case <-conn.Closing:
			log.Println("closing")

		case msg, isClosed := <-conn.Incoming:
			log.Println("received from incoming chan")

			if !isClosed {
				processMessage(conn, msg)
			}
		}
	}
}
