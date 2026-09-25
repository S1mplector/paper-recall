import QtQuick
Rectangle {
    id: control
    property string label: ""
    property string detail: ""
    property bool primary: false
    signal clicked()
    implicitHeight: 76
    radius: 10
    color: !enabled ? "#eeeeea" : area.pressed ? "#ccccca" : primary ? "#202420" : "white"
    border.width: primary ? 0 : 2
    border.color: "#b2b6ad"
    opacity: enabled ? 1 : 0.5
    Column {
        anchors.centerIn: parent
        spacing: 3
        Text { anchors.horizontalCenter: parent.horizontalCenter; text: control.label; font.pixelSize: 23; font.bold: true; color: control.primary ? "white" : "#202420" }
        Text { visible: text !== ""; anchors.horizontalCenter: parent.horizontalCenter; text: control.detail; font.pixelSize: 19; color: control.primary ? "white" : "#555b52" }
    }
    MouseArea { id: area; anchors.fill: parent; onClicked: control.clicked() }
}
