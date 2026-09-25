import QtQuick
import net.asivery.AppLoad 1.0

Item {
    id: transport
    signal response(var message)
    property string buffer: ""
    function send(message) { endpoint.sendMessage(1, JSON.stringify(message)); }
    function stop() { endpoint.terminate(); }
    AppLoad {
        id: endpoint
        applicationID: "paper-recall"
        onMessageReceived: (type, contents) => {
            if (type === 100) transport.buffer = "";
            else if (type === 101) transport.buffer += contents;
            else if (type === 102) {
                try { transport.response(JSON.parse(transport.buffer)); }
                catch (e) { transport.response({error: "Could not read the app response. Close and reopen Paper Recall."}); }
                transport.buffer = "";
            }
        }
    }
}
