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

const sfuWs = new WebSocket("ws://localhost:8083");

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
        data.sfuPeerConnection = new RTCPeerConnection({ iceServers: [] });

        data.mediaStream.getTracks().forEach(track => {
            console.log(track);
            data.sfuPeerConnection.addTrack(track, data.mediaStream);
        });

        data.sfuPeerConnection.ontrack = (ev) => {
            console.log("Received track: ", ev);
        }

        data.sfuPeerConnection.createOffer()
            .then(offer => {
                data.sfuPeerConnection.setLocalDescription(offer);

                sfuWs.onmessage = ev => {
                    console.log(ev);
                    
                    const message = JSON.parse(ev.data);

                    if (message.answer) {
                        data.sfuPeerConnection.setRemoteDescription(message.answer);
                    }
                };

                const msg = { action: "broadcast", value: data.roomName, offer };
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