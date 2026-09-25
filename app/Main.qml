import QtQuick
import QtQuick.Controls

Rectangle {
    id: app
    anchors.fill: parent
    color: "#f7f7f2"
    signal close()
    function unloading() { if (bridge.item) bridge.item.stop(); }
    property real unit: width / 640
    property string transportSource: "DeviceTransport.qml"
    property string page: "home"
    property var decks: []
    property var folders: []
    property var entries: []
    property string currentFolder: ""
    property var current: null
    property bool revealed: false
    property bool practice: false
    property int practiceTotal: 0
    property string selectedDeck: ""
    property string selectedName: ""
    property string error: ""
    property string notice: ""
    property bool clockWarning: false
    property int reviewedToday: 0
    property int dueCount: 0
    property int sessionCount: 0
    property bool canUndo: false
    property string editId: ""
    property bool busy: true
    property string pending: ""
    property string folderDeck: ""
    property real nextDue: 0
    property bool selecting: false
    property var selectedIDs: []
    property string deletionSummary: ""
    property var menuDeck: null
    function openDeckMenu(deck) { if (!busy) menuDeck = deck; }
    function deckAction(action) {
        if (!menuDeck || busy) return;
        var deck = menuDeck; menuDeck = null;
        if (action === "Review") start(deck.id, deck.name);
        else if (action === "Practice") { selectedDeck = deck.id; selectedName = deck.name; startPractice(); }
        else if (action === "Move to folder") folderEditor(deck);
        else if (action === "Select deck") { selecting = true; if (selectedIDs.indexOf(deck.id) < 0) toggleDeck(deck.id); }
        else if (action === "Delete deck") { selecting = true; selectedIDs = [deck.id]; confirmDeletion(); }
    }
    function toggleDeck(id) {
        var ids = selectedIDs.slice(), index = ids.indexOf(id);
        if (index < 0) ids.push(id); else ids.splice(index, 1);
        selectedIDs = ids;
    }
    function cancelSelection() { selecting = false; selectedIDs = []; page = "home"; }
    function confirmDeletion() {
        var chosen = decks.filter(function(d) { return selectedIDs.indexOf(d.id) >= 0; });
        if (!chosen.length) return;
        var count = chosen.reduce(function(n, d) { return n + d.total; }, 0);
        deletionSummary = chosen.length + " deck(s), " + count + " cards\n\n" + chosen.map(function(d) { return (d.folder ? d.folder + "/" : "") + d.name; }).join("\n") + "\n\nTheir cards and review progress will be removed. This cannot be undone in the app. Export first if you want to keep the cards.";
        page = "delete";
    }

    Loader { id: bridge; source: app.transportSource; onLoaded: { app.busy = false; app.request("status"); } }
    Connections { target: bridge.item; function onResponse(message) { app.accept(message); } }
    Timer { id: timeout; interval: 15000; onTriggered: { app.busy = false; app.error = "No response from storage. Close and reopen the app before trying again. A completed save will be preserved."; } }
    Timer { interval: 30000; running: true; repeat: true; onTriggered: if (!app.busy && (app.page === "home" || (app.page === "review" && !app.current))) app.refresh() }
    function request(action, fields) {
        if (busy || !bridge.item) return;
        var message = fields || {};
        message.practice = practice; message.practiceIndex = message.practiceIndex === undefined ? sessionCount : message.practiceIndex;
        message.action = action; message.deck = message.deck === undefined ? selectedDeck : message.deck;
        message.token = Date.now() + "-" + Math.random().toString(36).slice(2);
        pending = message.token; busy = true; timeout.restart(); bridge.item.send(message);
    }
    function accept(message) {
        if (message.token && message.token !== pending) return;
        timeout.stop(); busy = false;
        if (message.error) { error = message.error; return; }
        decks = message.decks || []; folders = message.folders || []; dueCount = message.dueCount || 0;
        reviewedToday = message.reviewedToday || 0; canUndo = !!message.canUndo;
        notice = message.notice || ""; clockWarning = !!message.clockWarning; nextDue = message.nextDue || 0;
        current = message.current || null;
        practiceTotal = message.practiceTotal || 0;
        if (message.action === "practiceNext") { sessionCount++; revealed = false; }
        if (message.action === "review") { sessionCount++; revealed = false; }
        if (message.action === "undo") { sessionCount = Math.max(0, sessionCount - 1); revealed = true; }
        if (message.action === "save" || message.action === "folder" || message.action === "moveDeck") { Qt.inputMethod.hide(); page = "home"; }
        if (message.action === "deleteDecks") { cancelSelection(); selectedDeck = ""; selectedName = ""; notice = "Selected decks deleted."; }
        buildEntries();
    }
    function buildEntries() {
        var prefix = currentFolder ? currentFolder + "/" : "", folderMap = {}, out = [];
        function addFolder(path) {
            if (path.indexOf(prefix) !== 0 || path === currentFolder) return;
            var segment = path.slice(prefix.length).split("/")[0];
            if (segment) folderMap[segment] = {kind: "folder", name: segment, path: prefix + segment, total: 0, due: 0};
        }
        folders.forEach(addFolder); decks.forEach(function(d) { addFolder(d.folder || ""); });
        decks.forEach(function(d) {
            var path = d.folder || "";
            if (path === currentFolder) out.push({kind:"deck", name:d.name, id:d.id, folder:path, total:d.total, due:d.due});
            else if (path.indexOf(prefix) === 0) {
                var segment = path.slice(prefix.length).split("/")[0];
                if (folderMap[segment]) { folderMap[segment].total += d.total; folderMap[segment].due += d.due; }
            }
        });
        var dirs = Object.keys(folderMap).sort().map(function(k) { return folderMap[k]; });
        entries = dirs.concat(out);
    }
    function refresh() { request("status"); }
    function start(deck, name) { practice = false; selectedDeck = deck; selectedName = name || "All decks"; sessionCount = 0; revealed = false; page = "review"; request("status"); }
    function startPractice() { practice = true; sessionCount = 0; revealed = false; page = "review"; request("status"); }
    function nextPractice() { if (!busy && practice && current && revealed) request("practiceNext", {practiceIndex: sessionCount + 1}); }
    function rate(rating) {
        if (!current || !revealed || busy || practice) return;
        request("review", {id:current.id, expected:current.schedule.reviews, rating:rating});
    }
    function undoReview() { request("undo"); }
    function edit(card) {
        editId = card ? card.id : ""; deckInput.text = card ? card.deck : "My cards";
        frontInput.text = card ? card.front : ""; backInput.text = card ? card.back : ""; page = "edit";
    }
    function saveCard() {
        if (!deckInput.text.trim() || !frontInput.text.trim() || !backInput.text.trim()) { error = "Fill in the deck, question, and answer."; return; }
        request("save", {id:editId, deckName:deckInput.text.trim(), front:frontInput.text.trim(), back:backInput.text.trim(), folder:currentFolder});
    }
    function folderEditor(deck) {
        folderDeck = deck ? deck.id : "";
        folderInput.text = deck ? (deck.folder || "") : currentFolder;
        page = "folder";
    }

    Item {
        id: canvas
        width: 640; height: app.height / app.unit
        scale: app.unit; transformOrigin: Item.TopLeft
        Item {
            id: header; x: 32; y: 32; width: 576; height: 76
            Text { text: "PAPER / RECALL"; font.pixelSize: 20; font.letterSpacing: 3; font.bold: true; color: "#394535"; anchors.verticalCenter: parent.verticalCenter }
            RecallButton { x: 454; width: 122; height: 62; label: app.page === "home" ? "Close" : "Home"; onClicked: { Qt.inputMethod.hide(); if (app.page === "home") app.close(); else { app.page = "home"; app.practice = false; app.refresh(); } } }
        }
        Rectangle { x: 32; y: 122; width: 576; height: 2; color: "#202420" }

        Item {
            visible: app.page === "home"; x: 32; y: 158; width: 576; height: canvas.height - y - 26
            Text { id: title; text: "Decks"; font.pixelSize: 46; font.family: "Noto Serif"; color: "#202420" }
            RecallButton { x: 400; y: 0; width: 176; height: 62; label: app.selecting ? "Cancel" : "Select"; enabled: !app.busy && app.decks.length > 0; onClicked: { if (app.selecting) app.cancelSelection(); else { app.selecting = true; app.selectedIDs = []; } } }
            Rectangle {
                y: 85; width: parent.width; height: 166; radius: 14; color: "#e7eadf"
                Text { x: 24; y: 21; text: app.dueCount; font.pixelSize: 64; font.bold: true; color: "#202420" }
                Text { x: 24; y: 102; text: "cards ready"; font.pixelSize: 23; color: "#394535" }
                Text { x: 340; y: 33; text: app.reviewedToday; font.pixelSize: 45; color: "#202420" }
                Text { x: 340; y: 102; text: "reviews today"; font.pixelSize: 23; color: "#394535" }
            }
            RecallButton { y: 270; width: parent.width; height: 82; primary: true; label: app.dueCount ? "Start reviewing" : "Practice cards"; enabled: app.decks.length > 0 && !app.busy && !app.selecting; onClicked: { if (app.dueCount) app.start("", "All decks"); else { app.selectedDeck = ""; app.selectedName = "All decks"; app.startPractice(); } } }
            Text { y: 382; width: 370; text: app.currentFolder || "ALL FOLDERS"; elide: Text.ElideLeft; font.pixelSize: 20; font.bold: true; color: "#555b52" }
            RecallButton { y: 365; anchors.right: parent.right; width: 152; height: 62; visible: app.currentFolder !== ""; label: "Up"; enabled: !app.busy; onClicked: { if (app.currentFolder) { app.currentFolder = app.currentFolder.split("/").slice(0,-1).join("/"); app.buildEntries(); } else app.folderEditor(null); } }
            ListView {
                id: deckList; x: 0; y: 446; width: parent.width; height: Math.max(80, parent.height - y - 226); clip: true; spacing: 12
                model: app.entries
                delegate: Item {
                    id: entry
                    required property var modelData
                    property bool isFolder: modelData.kind === "folder"
                    width: deckList.width; height: 108
                    Rectangle { visible: entry.isFolder; x: 0; y: 0; width: 134; height: 28; radius: 7; color: "#dce2d3"; border.color: "#89937e"; border.width: 2 }
                    Rectangle { y: entry.isFolder ? 13 : 0; width: parent.width; height: parent.height - y; radius: 9; color: entry.isFolder ? "#e7eadf" : app.selectedIDs.indexOf(modelData.id) >= 0 ? "#e7eadf" : "white"; border.color: entry.isFolder ? "#89937e" : "#b2b6ad"; border.width: entry.isFolder ? 2 : 1 }
                    Item { visible: entry.isFolder; x: 20; y: 36; width: 48; height: 38
                        Rectangle { width: 22; height: 14; radius: 3; color: "#657359" }
                        Rectangle { y: 8; width: 48; height: 30; radius: 4; color: "#657359" }
                    }
                    Text { x: entry.isFolder ? 86 : 20; y: entry.isFolder ? 26 : 17; width: parent.width - x - 130; text: modelData.name; textFormat: Text.PlainText; elide: Text.ElideRight; font.pixelSize: 27; font.bold: true; color: "#202420" }
                    Text { x: entry.isFolder ? 86 : 20; y: 65; text: modelData.total + " cards · " + modelData.due + " due"; font.pixelSize: 21; color: "#555b52" }
                    Text { visible: entry.isFolder; anchors.right: parent.right; anchors.rightMargin: 28; y: 32; text: "›"; font.pixelSize: 42; color: "#394535" }
                    MouseArea {
                        anchors.fill: parent; enabled: !app.busy; pressAndHoldInterval: 600
                        property bool held: false
                        onPressed: held = false
                        onPressAndHold: { if (!entry.isFolder) { held = true; app.openDeckMenu(modelData); } }
                        onClicked: {
                            if (held) return;
                            if (entry.isFolder) { app.currentFolder = modelData.path; app.buildEntries(); }
                            else if (app.selecting) app.toggleDeck(modelData.id);
                            else app.start(modelData.id, modelData.name);
                        }
                    }
                    RecallButton { visible: !entry.isFolder; anchors.right: parent.right; anchors.rightMargin: 12; y: 23; width: 105; height: 62; label: app.selecting ? (app.selectedIDs.indexOf(modelData.id) >= 0 ? "✓" : "Select") : "•••"; enabled: !app.busy; onClicked: { if (app.selecting) app.toggleDeck(modelData.id); else app.openDeckMenu(modelData); } }
                }
                ScrollBar.vertical: ScrollBar {}
            }
            Row { visible: !app.selecting; anchors.bottom: createCard.top; anchors.bottomMargin: 14; spacing: 12
                RecallButton { width: 184; height: 68; label: "Import"; enabled: !app.busy; onClicked: app.request("import") }
                RecallButton { width: 184; height: 68; label: "Export"; enabled: !app.busy; onClicked: app.request("export") }
                RecallButton { width: 184; height: 68; label: "+ Folder"; enabled: !app.busy; onClicked: app.folderEditor(null) }
            }
            Text { visible: app.selecting; anchors.bottom: createCard.top; anchors.bottomMargin: 24; width: parent.width; text: app.selectedIDs.length + " selected. Tap decks to select.\nYou can browse folders to select more."; wrapMode: Text.Wrap; font.pixelSize: 22; color: "#394535" }
            RecallButton { id: createCard; anchors.bottom: parent.bottom; width: parent.width; height: 76; label: app.selecting ? "Delete selected (" + app.selectedIDs.length + ")" : "+  Create a card"; enabled: !app.busy && (!app.selecting || app.selectedIDs.length > 0); onClicked: { if (app.selecting) app.confirmDeletion(); else app.edit(null); } }
        }

        Item {
            visible: app.page === "review"; x: 32; y: 151; width: 576; height: canvas.height - y - 28
            Text { width: 410; text: app.selectedName || "All decks"; elide: Text.ElideRight; font.pixelSize: 27; font.bold: true; color: "#202420" }
            Text { anchors.right: parent.right; y: 5; text: app.sessionCount + (app.practice ? " practiced" : " reviewed"); font.pixelSize: 20; color: "#555b52" }
            Rectangle {
                x: 0; y: 67; width: parent.width; height: parent.height - 274; radius: 14; color: "white"; border.color: "#b2b6ad"
                Text { x: 28; y: 24; text: !app.current ? "SESSION COMPLETE" : (app.practice ? "PRACTICE · " : "") + (app.revealed ? "ANSWER" : "QUESTION"); font.pixelSize: 18; font.letterSpacing: 2; color: "#555b52" }
                Flickable {
                    x: 28; y: 78; width: parent.width - 56; height: parent.height - 113; clip: true; contentHeight: cardText.height
                    Text {
                        id: cardText; width: parent.width
                        text: !app.current ? "No cards due now." + (app.nextDue ? "\n\nNext review: " + new Date(app.nextDue).toLocaleString(Qt.locale(), "ddd d MMM, HH:mm") : "")
                            : app.revealed ? app.current.back : app.current.front
                        textFormat: Text.PlainText; wrapMode: Text.Wrap; font.pixelSize: 34; lineHeight: 1.2; color: "#202420"
                    }
                    MouseArea { anchors.fill: parent; onClicked: if (app.current && !app.revealed) app.revealed = true }
                    ScrollBar.vertical: ScrollBar {}
                }
            }
            RecallButton { visible: !!app.current && !app.revealed; anchors.bottom: footer.top; anchors.bottomMargin: 18; width: parent.width; height: 98; primary: true; label: "Show answer"; detail: ""; enabled: !app.busy; onClicked: app.revealed = true }
            Row {
                visible: !!app.current && app.revealed && !app.practice; anchors.bottom: footer.top; anchors.bottomMargin: 18; spacing: 10
                Repeater {
                    model: ["again", "hard", "good", "easy"]
                    delegate: RecallButton {
                        required property string modelData
                        width: 136.5; height: 98; primary: modelData === "good"
                        label: modelData.charAt(0).toUpperCase() + modelData.slice(1)
                        detail: app.current ? (app.current.labels[modelData] || "") : ""
                        enabled: !app.busy && !app.clockWarning
                        onClicked: app.rate(modelData)
                    }
                }
            }
            RecallButton { visible: !!app.current && app.revealed && app.practice; anchors.bottom: footer.top; anchors.bottomMargin: 18; width: parent.width; height: 98; primary: true; label: "Next card"; detail: "Practice · schedule unchanged"; enabled: !app.busy; onClicked: app.nextPractice() }
            RecallButton { visible: !app.current; anchors.bottom: footer.top; anchors.bottomMargin: 18; width: parent.width; height: 98; primary: true; label: "Practice again"; detail: "Review freely · schedule unchanged"; enabled: !app.busy && app.decks.some(function(d) { return !app.selectedDeck || d.id === app.selectedDeck; }); onClicked: app.startPractice() }
            Row {
                id: footer; anchors.bottom: parent.bottom; spacing: 16
                RecallButton { width: 280; height: 66; label: app.practice ? "Finish practice" : "Undo rating"; enabled: !app.busy && (app.practice || app.canUndo); onClicked: { if (app.practice) { app.practice = false; app.page = "home"; app.refresh(); } else app.undoReview(); } }
                RecallButton { width: 280; height: 66; label: "Edit card"; enabled: !!app.current && !app.busy; onClicked: app.edit(app.current) }
            }
        }

        Item {
            visible: app.page === "delete"; x: 32; y: 158; width: 576; height: canvas.height - y - 28
            Text { text: "Delete decks?"; font.pixelSize: 42; color: "#202420" }
            Flickable { y: 90; width: parent.width; height: parent.height - 200; contentHeight: deletionText.height; clip: true
                Text { id: deletionText; width: parent.width; text: app.deletionSummary; textFormat: Text.PlainText; wrapMode: Text.Wrap; font.pixelSize: 26; color: "#202420" }
                ScrollBar.vertical: ScrollBar {}
            }
            Row { anchors.bottom: parent.bottom; spacing: 16
                RecallButton { width: 280; label: "Cancel"; enabled: !app.busy; onClicked: app.page = "home" }
                RecallButton { width: 280; label: "Delete decks"; primary: true; enabled: !app.busy; onClicked: app.request("deleteDecks", {deckIds: app.selectedIDs, deck: ""}) }
            }
        }

        Flickable {
            id: editor; visible: app.page === "edit"; x: 32; y: 150; width: 576; height: canvas.height - y - 25 - (Qt.inputMethod.visible ? Qt.inputMethod.keyboardRectangle.height / app.unit : 0)
            clip: true; contentHeight: editColumn.height
            Column {
                id: editColumn; width: parent.width; spacing: 18
                Text { text: app.editId ? "Edit card" : "Create card"; font.pixelSize: 39; font.family: "Noto Serif"; color: "#202420" }
                Text { text: "DECK"; font.pixelSize: 19; color: "#555b52" }
                TextField { id: deckInput; readOnly: app.editId !== ""; width: parent.width; height: 64; font.pixelSize: 25; selectByMouse: true }
                Text { text: "QUESTION"; font.pixelSize: 19; color: "#555b52" }
                TextArea { id: frontInput; width: parent.width; height: Math.max(170, implicitHeight); font.pixelSize: 27; wrapMode: TextEdit.Wrap; selectByMouse: true; textFormat: TextEdit.PlainText; background: Rectangle { color: "white"; border.color: "#b2b6ad"; radius: 6 } }
                Text { text: "ANSWER"; font.pixelSize: 19; color: "#555b52" }
                TextArea { id: backInput; width: parent.width; height: Math.max(200, implicitHeight); font.pixelSize: 27; wrapMode: TextEdit.Wrap; selectByMouse: true; textFormat: TextEdit.PlainText; background: Rectangle { color: "white"; border.color: "#b2b6ad"; radius: 6 } }
                RecallButton { width: parent.width; height: 80; primary: true; label: "Save card"; enabled: !app.busy; onClicked: app.saveCard() }
                Text { width: parent.width; text: "Saved on this tablet. Review progress stays with your card when you edit it."; wrapMode: Text.Wrap; font.pixelSize: 20; color: "#555b52" }
            }
            ScrollBar.vertical: ScrollBar {}
        }
        Column {
            visible: app.page === "folder"; x: 32; y: 175; width: 576; spacing: 28
            Text { text: app.folderDeck ? "Move deck" : "Create folder"; font.pixelSize: 42; color: "#202420" }
            Text { width: parent.width; text: app.folderDeck ? "Enter a folder path. Leave empty to move this deck to the top level." : "Enter a folder name. Use / for nested folders, such as Languages/French."; wrapMode: Text.Wrap; font.pixelSize: 24; color: "#555b52" }
            TextField { id: folderInput; width: parent.width; height: 76; font.pixelSize: 26; selectByMouse: true }
            RecallButton { width: parent.width; primary: true; label: app.folderDeck ? "Move deck" : "Create folder"; enabled: !app.busy; onClicked: app.request(app.folderDeck ? "moveDeck" : "folder", {deck:app.folderDeck, folder:folderInput.text}) }
        }
        Rectangle {
            visible: app.notice !== "" || app.clockWarning; x: 32; y: 124; width: 576; height: Math.min(300, noticeText.implicitHeight + 40); color: "#e7eadf"; z: 5
            Text { id: noticeText; x: 15; y: 12; width: parent.width - 60; text: app.clockWarning ? "Check the tablet date and time before reviewing." : app.notice; wrapMode: Text.Wrap; font.pixelSize: 21; color: "#202420" }
            Text { anchors.right: parent.right; anchors.rightMargin: 12; y: 8; text: "×"; font.pixelSize: 28 }
            MouseArea { anchors.fill: parent; onClicked: app.notice = "" }
        }
        Rectangle {
            visible: app.menuDeck !== null; anchors.fill: parent; color: "#b3f7f7f2"; z: 8
            MouseArea { anchors.fill: parent; onClicked: app.menuDeck = null }
            Rectangle {
                x: 32; anchors.verticalCenter: parent.verticalCenter; width: 576; height: 686; radius: 16; color: "#f7f7f2"; border.color: "#394535"; border.width: 2
                MouseArea { anchors.fill: parent }
                Column { x: 24; y: 24; width: parent.width - 48; spacing: 14
                    Text { width: parent.width; text: app.menuDeck ? app.menuDeck.name : ""; textFormat: Text.PlainText; elide: Text.ElideRight; font.pixelSize: 32; font.bold: true; color: "#202420" }
                    Text { width: parent.width; text: app.menuDeck ? (app.menuDeck.folder || "All folders") : ""; textFormat: Text.PlainText; elide: Text.ElideLeft; font.pixelSize: 21; color: "#555b52" }
                    Repeater { model: ["Review", "Practice", "Move to folder", "Select deck", "Delete deck", "Cancel"]
                        delegate: RecallButton { required property string modelData; width: 528; height: 76; label: modelData; enabled: !app.busy; onClicked: app.deckAction(modelData) }
                    }
                }
            }
        }
        Text { visible: app.busy; x: 250; y: 108; text: "Saving / loading…"; font.pixelSize: 16; color: "#555b52"; z: 6 }
        Rectangle {
            visible: app.error !== ""; anchors.fill: parent; color: "#f7f7f2"; z: 10
            Column { x: 32; y: 180; width: 576; spacing: 35
                Text { width: parent.width; text: app.error; wrapMode: Text.Wrap; font.pixelSize: 25; color: "#202420" }
                RecallButton { width: parent.width; label: "Dismiss"; onClicked: app.error = "" }
            }
        }
    }
}
