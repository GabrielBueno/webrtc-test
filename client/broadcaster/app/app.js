const data = {
    devicesAvailable: {
        videoin:  [],
        audioin:  [],
        audioout: []
    },
    selectedVideoInput: null,
    selectedAudioInput: null,
    roomName:           "",
    mediaStream:        null,
    sfuPeerConnection:  null,
    sfuWsConnection:    null
};

const sfuWs = new WebSocket("wss://service.gbrl.dev/signal");

function log(msg) {
    const logList = document.getElementById("loglist");
    const log     = document.createElement("p");

    log.innerHTML = msg;

    logList.appendChild(log);
    console.log(msg);
}

function getSelectedDevicesConstraints() {
    const _constraintDevId = (dev) => dev ? { "deviceId": dev.deviceId } : false;

    return {
        "audio": _constraintDevId(data.selectedAudioInput),
        "video": _constraintDevId(data.selectedVideoInput)
    };
}

function getAvailableDevices() {
    if (!navigator || !navigator.mediaDevices)
        log("no support");

    navigator.mediaDevices.enumerateDevices()
        .then(devInfos => {
            data.devicesAvailable.videoin  = devInfos.filter(dev => dev.kind === "videoinput");
            data.devicesAvailable.audioin  = devInfos.filter(dev => dev.kind === "audioinput");
            data.devicesAvailable.audioout = devInfos.filter(dev => dev.kind === "audiooutput");
            
            if (data.devicesAvailable.videoin.length > 0)
                data.selectedVideoInput = data.devicesAvailable.videoin[0];

            if (data.devicesAvailable.audioin.length > 0)
                data.selectedAudioInput = data.devicesAvailable.audioin[0];

            log("devs found");
        })
        .catch(() => log(errorCallback));
}

function createStream() {
    resetStream().then(() => {
        data.sfuPeerConnection = new RTCPeerConnection({ 
            iceServers: [
                { urls: "stun:stun1.l.google.com:19302" },
                { urls: "stun:stun2.l.google.com:19302" },
                { urls: "stun:stun3.l.google.com:19302" },
                { urls: "stun:stun4.l.google.com:19302" },
                { urls: "stun:stun.stunprotocol.org:3478" },
            ] 
        });

        data.mediaStream.getTracks().forEach(track => {
            console.log(track);
            data.sfuPeerConnection.addTrack(track, data.mediaStream);
        });

        data.sfuPeerConnection.ontrack = (ev) => {
            console.log("Received track: ", ev);
        }

        data.sfuPeerConnection.onicecandidate = (ev) => {
            console.log(ev);

            if (ev) {
                const msg = { Intention: "send_ice", IceCandidate: ev }
                sfuWs.send(JSON.stringify(msg));
            }
        }

        data.sfuPeerConnection.createOffer()
            .then(offer => {
                data.sfuPeerConnection.setLocalDescription(offer);
                console.log(offer);

                sfuWs.onmessage = ev => {
                    const message = JSON.parse(ev.data);
                    console.log(message)

                    if (message.Intention === "answer") {
                        data.sfuPeerConnection.setRemoteDescription(message.Sdp);

                        const finishMsg = { Intention: "finish" };
                        sfuWs.send(JSON.stringify(finishMsg));
                    }

                    if (message.Intention === "send_ice" && message.IceCandidate) {
                        data.sfuPeerConnection.addIceCandidate(message.IceCandidate)
                    }
                };

                const msg = { Intention: "broadcast", Detail: data.roomName, Sdp: offer };
                sfuWs.send(JSON.stringify(msg));
            })
            .catch(err => log(err))
    });
};

function resetStream() {
    const constraints = getSelectedDevicesConstraints();
    const video       = document.getElementById("mystream");

    return navigator.mediaDevices.getUserMedia(constraints)
        .then(stream => {
            data.mediaStream = stream;
            // video.srcObject  = data.mediaStream;
        })
};

getAvailableDevices();
// navigator.mediaDevices.addEventListener("devicechange", _ => getAvailableDevices());

const vue = new Vue({
    el:   "main",
    data: data,
    methods: {
        resetStream,
        createStream
    }
});