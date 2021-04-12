package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

const (
	RTCP_PLI_INTERVAL = time.Second * 3
)

type Peer struct {
	Name       string
	PeerConn   *webrtc.PeerConnection
	SignalConn SignallingConnection
}

type ConnGateway struct {
	NewPeer chan *Peer
}

type Room struct {
	Name        string
	Broadcaster *Peer
	Watchers    []*Peer

	NewWatcher chan *Peer
	NewTrack   chan *webrtc.TrackLocalStaticRTP
}

type Hub struct {
	rooms map[string]*Room
}

func NewHub() Hub {
	return Hub{rooms: make(map[string]*Room)}
}

func NewConnGateway() ConnGateway {
	return ConnGateway{NewPeer: make(chan *Peer)}
}

func (gateway *ConnGateway) HandleNewPeerConnection(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Printf("err: couldn't estabilish a websocket connection: %s", err)
		return
	}

	log.Printf("received connection from %s\n", r.Header.Get("Origin"))

	peer := Peer{
		PeerConn: nil,
		SignalConn: SignallingConnection{
			WsConn:   conn,
			Incoming: make(chan SignalMessage),
			Sending:  make(chan SignalMessage),
			IsOpen:   true,
		},
	}

	go peer.SignalConn.listenIncomingMessages()
	go peer.SignalConn.listenSendingMessages()

	gateway.NewPeer <- &peer
}

func (hub *Hub) Negotiate(peer *Peer) {
	for peer.SignalConn.IsOpen {
		select {
		case <-peer.SignalConn.Closing:

		case msg, isOpened := <-peer.SignalConn.Incoming:
			if isOpened {
				hub.processMessage(peer, msg)
			}
		}
	}
}

func (hub *Hub) processMessage(peer *Peer, msg SignalMessage) {
	log.Printf("received msg: %v\n", msg)

	switch msg.Intention {
	case BROADCAST_INTENTION:
		err := hub.newBroadcast(peer, msg.Detail, msg.Sdp)

		if err != nil {
			denialMsg := SignalMessage{Intention: DENY_INTENTION, Detail: err.Error()}
			peer.SignalConn.Send(denialMsg)
		}

	case WATCH_INTENTION:
		err := hub.watch(peer, msg.Detail, msg.Sdp)

		if err != nil {
			denialMsg := SignalMessage{Intention: DENY_INTENTION, Detail: err.Error()}
			peer.SignalConn.Send(denialMsg)
		}

	case SEND_ICE_INTENTION:
		if peer.PeerConn == nil {
			log.Printf("received ice but PeerConn is nil.\n")
		}

		if peer.PeerConn != nil && msg.IceCandidate != nil {
			log.Printf("RECEIVING ICE\n")
			err := peer.PeerConn.AddICECandidate(*msg.IceCandidate)

			if err != nil {
				log.Printf("ERR: while adding ice candidate: %s\n", err)
			}
		}

	case FINISH_INTENTION:
		// conn.Sending <- SignalMessage{Intention: FINISH_INTENTION}

	default:
		log.Printf("ERR: invalid intention for negotiation: %s\n", msg.Intention)

		denialMsg := SignalMessage{
			Intention: DENY_INTENTION,
			Detail:    msg.Intention + " is an invalid intention",
		}

		peer.SignalConn.Send(denialMsg)
	}
}

func (hub *Hub) newBroadcast(peer *Peer, roomName string, offer *webrtc.SessionDescription) error {
	_, roomAlreadyExists := hub.rooms[roomName]

	if roomAlreadyExists {
		return errors.New("A room with this name already exists")
	}

	peerConn, err := webrtc.NewPeerConnection(defaultBroadcasterPeerConnection())

	if err != nil {
		return errors.New("Couldn't create a connection to the broadcaster peer")
	}

	peer.PeerConn = peerConn
	_, err = peer.PeerConn.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio)

	if err != nil {
		log.Printf("ERR: while adding audio transceiver: %s\n", err)
		return errors.New("Couldn't add audio transceiver")
	}

	newRoom := Room{
		Name:        roomName,
		Broadcaster: peer,
		NewWatcher:  make(chan *Peer),
		NewTrack:    make(chan *webrtc.TrackLocalStaticRTP),
	}

	// On Track Listener
	peer.PeerConn.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("ONTRACK\n")
		// Package Loss Indicator sender
		go func() {
			ticker := time.NewTicker(RTCP_PLI_INTERVAL)

			for range ticker.C {
				pli := rtcp.PictureLossIndication{MediaSSRC: uint32(remoteTrack.SSRC())}
				packet := []rtcp.Packet{&pli}

				pliSendErr := peer.PeerConn.WriteRTCP(packet)

				if pliSendErr != nil {
					log.Printf("ERR: while sending pli: %s\n", pliSendErr)
				}
			}
		}()

		// Creates local track
		codecCapability := remoteTrack.Codec().RTPCodecCapability
		localTrack, newTrackErr := webrtc.NewTrackLocalStaticRTP(codecCapability, remoteTrack.ID(), remoteTrack.StreamID())

		if newTrackErr != nil {
			log.Printf("ERR: while creating local track: %s\n", newTrackErr)
			return
		}

		newRoom.NewTrack <- localTrack

		rtpBuffer := make([]byte, 1400)

		// Reads & writes
		for {
			readLen, _, readErr := remoteTrack.Read(rtpBuffer)

			if readErr != nil {
				log.Printf("err: while reading track: %s\n", readErr)
				return
			}

			_, writeErr := localTrack.Write(rtpBuffer[:readLen])

			if writeErr != nil && !errors.Is(writeErr, io.ErrClosedPipe) {
				log.Printf("err: while writing local track: %s\n", writeErr)
				return
			}
		}
	})

	err = peer.PeerConn.SetRemoteDescription(*offer)

	if err != nil {
		log.Printf("err: while accepting broadcast offer: %s\n", err)
		return errors.New("Couldn't accept offer")
	}

	answer, err := peer.PeerConn.CreateAnswer(nil)

	if err != nil {
		log.Printf("err: while creating answer: %s\n", err)
		return errors.New("Couldn't create answer")
	}

	peer.PeerConn.OnICECandidate(func(iceCandidate *webrtc.ICECandidate) {
		if iceCandidate != nil {
			log.Printf("OnICECandidate BROADCASTER: sending ice candidate.\n")

			iceCandidateInit := iceCandidate.ToJSON()
			peer.SignalConn.Send(SignalMessage{Intention: SEND_ICE_INTENTION, IceCandidate: &iceCandidateInit})
		}
	})

	err = peer.PeerConn.SetLocalDescription(answer)

	if err != nil {
		log.Printf("err: while setting local description for broadcast: %s\n", err)
		return errors.New("Server error: couldn't set local sdp")
	}

	answerMsg := SignalMessage{Intention: ANSWER_INTENTION, Sdp: &answer}
	peer.SignalConn.Send(answerMsg)

	hub.rooms[roomName] = &newRoom

	go newRoom.listenForWatchers()

	return nil
}

func (hub *Hub) watch(peer *Peer, roomName string, offer *webrtc.SessionDescription) error {
	room, roomExists := hub.rooms[roomName]

	if !roomExists {
		return errors.New("This room doesn't exist")
	}

	peerConn, err := webrtc.NewPeerConnection(defaultWatcherPeerConnection())

	if err != nil {
		log.Printf("err: while creating watcher peer conn: %s\n", err)
		return errors.New("Couldn't create watcher peer conn")
	}

	peer.PeerConn = peerConn

	localTrack := <-room.NewTrack
	rtpSender, err := peer.PeerConn.AddTrack(localTrack)

	if err != nil {
		log.Printf("err: while adding track to watcher conn: %s\n", err)
		return errors.New("Couldn't add track")
	}

	go func() {
		rtcpBuffer := make([]byte, 1500)

		for {
			_, _, rtcpReadErr := rtpSender.Read(rtcpBuffer)

			if rtcpReadErr != nil {
				log.Printf("err: reading rtpcBuffer from watcher rtpSender: %s\n", rtcpReadErr)
				return
			}
		}
	}()

	// go func() {
	// 	for {
	// 		localTrack := <-room.newTrack

	// 		rtpSender, err := watcherPeerConn.AddTrack(localTrack)

	// 		if err != nil {
	// 			log.Printf("err: while adding track to watcher conn: %s\n", err)
	// 			return
	// 		}

	// 		go func() {
	// 			rtcpBuffer := make([]byte, 1500)

	// 			for {
	// 				_, _, rtcpReadErr := rtpSender.Read(rtcpBuffer)

	// 				if rtcpReadErr != nil {
	// 					log.Printf("err: reading rtpcBuffer from watcher rtpSender: %s\n", rtcpReadErr)
	// 					return
	// 				}
	// 			}
	// 		}()
	// 	}
	// }()

	if err != nil {
		log.Printf("err: while creating peer conn for watcher: %s\n", err)
		return errors.New("Couldn't create peer connection")
	}

	err = peer.PeerConn.SetRemoteDescription(*offer)

	if err != nil {
		log.Printf("err: while accepting watcher offer: %s\n", err)
		return errors.New("Couldn't accept offer")
	}

	answer, err := peer.PeerConn.CreateAnswer(nil)

	if err != nil {
		log.Printf("err: while creating watcher's answer: %s\n", err)
		return errors.New("Couldn't create answer")
	}

	peer.PeerConn.OnICECandidate(func(iceCandidate *webrtc.ICECandidate) {
		if iceCandidate != nil {
			log.Printf("OnICECandidate WATCHER: sending ice candidate.\n")

			iceCandidateInit := iceCandidate.ToJSON()
			peer.SignalConn.Send(SignalMessage{Intention: SEND_ICE_INTENTION, IceCandidate: &iceCandidateInit})
		}
	})

	err = peer.PeerConn.SetLocalDescription(answer)

	if err != nil {
		log.Printf("err: while setting local description: %s\n", err)
		return errors.New("Server error: couldn't accept local sdp")
	}

	answerMsg := SignalMessage{Intention: ANSWER_INTENTION, Sdp: &answer}
	peer.SignalConn.Send(answerMsg)

	room.NewWatcher <- peer

	return nil
}

func (room *Room) listenForWatchers() {
	for {
		select {
		case <-room.NewWatcher:
			log.Printf("new watcher accepted in room %s\n", room.Name)
		}
	}
}

func defaultBroadcasterPeerConnection() webrtc.Configuration {
	return webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun1.l.google.com:19302"}},
			{URLs: []string{"stun:stun.stunprotocol.org:3478"}},
		},
	}
}

func defaultWatcherPeerConnection() webrtc.Configuration {
	return webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun1.l.google.com:19302"}},
			{URLs: []string{"stun:stun.stunprotocol.org:3478"}},
		},
	}
}
