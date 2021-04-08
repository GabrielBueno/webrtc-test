const data = {
    roomName:          "",
    sfuPeerConnection: null
};

const sfuWs = new WebSocket("ws://localhost:8083/signal");

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
    data.sfuPeerConnection = new RTCPeerConnection({ iceServers: [] });

    data.sfuPeerConnection.ontrack = (ev) => {
        console.log(ev);
        document.getElementById("stream").srcObject = ev.streams[0];
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

            const msg = { intention: "invade", value: data.roomName, offer };
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