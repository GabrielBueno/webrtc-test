const WebSocket             = require("ws");
const { RTCPeerConnection } = require("wrtc");

class Watcher {
    constructor(id, peerconn) {
        this.id             = id;
        this.peerConnection = peerconn;
    }
}

class Room {
    constructor(name) {
        this.name                      = name;
        this.broadcasterPeerConnection = new RTCPeerConnection({ iceServers: [] });
        this.broadcasterOffer          = null;
        this.watchers                  = [];
        this.tracks                    = [];
    }

    addNewWatcher() {
        const watcherId  = this.watchers.length;
        const newWatcher = new Watcher(watcherId, new RTCPeerConnection({ iceServers: [] }));

        this.watchers.push(newWatcher);

        return newWatcher;
    }
}

const hub = new Map();
const wss = new WebSocket.Server({ port: 8083 });

const createRoom = (clientWs, roomName, clientOffer) => {
    if (hub.has(roomName)) {
        clientWs.send(JSON.stringify({ error: "Room already created" }));
        return;
    }

    console.log(roomName);
    console.log(clientOffer);

    const room = new Room(roomName)
    hub.set(roomName, room);

    room.broadcasterPeerConnection.setRemoteDescription(clientOffer);
    room.broadcasterOffer = clientOffer;

    room.broadcasterPeerConnection.ontrack = (ev) => {
        console.log("track found: ", ev);
        room.tracks.push(ev);
        // room.watchers.forEach(w => w.peerConnection.addTrack(ev.track));
    }

    room.broadcasterPeerConnection.createAnswer()
        .then(answer => {
            room.broadcasterPeerConnection.setLocalDescription(answer);

            clientWs.send(JSON.stringify({ answer }));
        });
};

const watchRoom = (clientWs, roomName, clientOffer) => {
    const room = hub.get(roomName);

    if (!room) {
        clientWs.send(JSON.stringify({ error: "No room found with this name" }));
        return;
    }

    const watcher = room.addNewWatcher();

    room.tracks.forEach(t => {
        console.log("sendig track: ", t);
        watcher.peerConnection.addTrack(t.track, t.streams[0]);
    });

    watcher.peerConnection.setRemoteDescription(clientOffer);

    watcher.peerConnection.createAnswer()
        .then(answer => {
            watcher.peerConnection.setLocalDescription(answer);

            clientWs.send(JSON.stringify({ answer }));
        });
};

wss.on("connection", ws => {
    ws.on("message", msg => {
        console.log(msg);
        const data = JSON.parse(msg);

        if (data.action === "broadcast")
            createRoom(ws, data.value, data.offer);

        if (data.action === "watch")
            watchRoom(ws, data.value, data.offer);
    });
});

console.log(RTCPeerConnection);
console.log("Listening on 8083...");