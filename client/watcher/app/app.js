const data = {
    roomName:          "",
    sfuPeerConnection: null
};

const sfuWs = new WebSocket("wss://service.gbrl.dev/signal");

// const watchStream = () => {
//     data.sfuPeerConnection = new RTCPeerConnection();

//     data.sfuPeerConnection.ontrack = (ev) => {
//         document.getElementById("stream").srcObject = ev.streams[0];
//     };

//     sfuWs.onmessage = (ev) => {
//         const message = JSON.parse(ev.data);

//         if (!message.offer)
//             return;

//         data.sfuPeerConnection.setRemoteDescription(message.offer);
//         data.sfuPeerConnection.createAnswer()
//             .then(answer => {
//                 data.sfuPeerConnection.setLocalDescription(answer);
//                 sfuWs.send(JSON.stringify({ answer }));
//             });
//     };

//     sfuWs.send(JSON.stringify({ action: "watch", value: data.roomName }));
// };

const watchStream = () => {
    data.sfuPeerConnection = new RTCPeerConnection({ 
        iceServers: [
            { urls: "stun:stun1.l.google.com:19302" },
            { urls: "stun:stun.stunprotocol.org:3478" },
        ] 
    });

    data.sfuPeerConnection.ontrack = (ev) => {
        console.log("received track", ev);
        
        const video = document.getElementById("stream");

        if (!video.srcObject)
            video.srcObject = ev.streams[0];
    }

    data.sfuPeerConnection.onicecandidate = (ev) => {
        console.log(ev);

        if (ev) {
            const msg = { Intention: "send_ice", IceCandidate: ev.candidate }
            sfuWs.send(JSON.stringify(msg));
        }
    }

    data.sfuPeerConnection.createOffer({ offerToReceiveAudio: true, offerToReceiveVideo: true })
        .then(offer => {
            data.sfuPeerConnection.setLocalDescription(offer);
            console.log(offer);

            sfuWs.onmessage = ev => {
                const message = JSON.parse(ev.data);
                console.log(message)

                if (message.Intention === "answer") {
                    data.sfuPeerConnection.setRemoteDescription(message.Sdp);

                    const finishMsg = { Intention: "finish" };
                    sfuWs.send(JSON.stringify(finishMsg))
                }

                if (message.Intention === "send_ice" && message.IceCandidate) {
                    data.sfuPeerConnection.addIceCandidate(message.IceCandidate)
                }
            };

            const msg = { Intention: "watch", Detail: data.roomName, Sdp: offer };
            sfuWs.send(JSON.stringify(msg));
        })
        .catch(err => log(err))
};

const vue = new Vue({
    el:   "main",
    data: data,
    methods: {
        watchStream
    }
});