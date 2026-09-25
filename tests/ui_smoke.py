import os,tempfile,socket,struct,subprocess,json,shutil
os.environ['QT_QPA_PLATFORM']='offscreen';os.environ['QT_QUICK_BACKEND']='software'
from PySide6.QtCore import QUrl,QResource,QObject,Signal,Slot,QPoint,Qt,QMetaObject
from PySide6.QtGui import QGuiApplication
from PySide6.QtQuick import QQuickView
from PySide6.QtTest import QTest
from pathlib import Path
root=str(Path(__file__).resolve().parents[1])
temp=tempfile.mkdtemp(prefix='recall-native-test-');data=temp+'/data';os.makedirs(data+'/imports')
shutil.copy(root+'/backend/welcome.recall',temp+'/welcome.recall')
subprocess.run([os.environ.get('GO','go'),'build','-o',temp+'/backend','./backend'],cwd=root,check=True)
transport=Path(temp+'/transport.qml')
transport.write_text('import QtQuick\nItem { id: transport; signal response(var message); function send(message) { testBackend.request(JSON.stringify(message)); } function stop() {} Connections { target: testBackend; function onDelivered(message) { transport.response(JSON.parse(message)); } } }')
server=socket.socket(socket.AF_UNIX,socket.SOCK_SEQPACKET);server.bind(temp+'/socket');server.listen(1)
proc=subprocess.Popen([temp+'/backend',temp+'/socket'],env={**os.environ,'PAPER_RECALL_DATA':data})
conn,_=server.accept();conn.settimeout(5)
class Bridge(QObject):
 delivered=Signal(str)
 @Slot(str)
 def request(self,request):
  b=request.encode();conn.send(struct.pack('<II',1,len(b)));conn.send(b);parts=[]
  while True:
   kind,length=struct.unpack('<II',conn.recv(8));payload=conn.recv(length) if length else b''
   if kind==101:parts.append(payload)
   if kind==102:break
  self.delivered.emit(b''.join(parts).decode())
app=QGuiApplication([]);bridge=Bridge();QResource.registerResource(root+'/build/paper-recall/resources.rcc')
view=QQuickView();view.rootContext().setContextProperty('testBackend',bridge);view.setInitialProperties({'transportSource':QUrl.fromLocalFile(str(transport)).toString()})
view.setResizeMode(QQuickView.SizeRootObjectToView);view.resize(640,1138);view.setSource(QUrl('qrc:/paper-recall/Main.qml'));view.show();QTest.qWait(500)
r=view.rootObject();assert r,view.errors();assert r.property('error')=='',r.property('error');assert r.property('dueCount')==6
view.grabWindow().save(temp+'/recall-v2-home.png')
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(300,465));QTest.qWait(100);assert r.property('page')=='review'
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(300,980));QTest.qWait(100);assert r.property('revealed')
view.grabWindow().save(temp+'/recall-v2-answer.png')
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(400,980));QTest.qWait(100)
assert r.property('reviewedToday')==1,r.property('error');assert r.property('dueCount')==5
QMetaObject.invokeMethod(r,'undoReview');QTest.qWait(100);assert r.property('reviewedToday')==0;assert r.property('dueCount')==6
# Import a nested folder through actual backend request path.
d=json.loads(open(root+'/backend/welcome.recall').read());d['deck']={'id':'french','name':'French','folder':'Languages/French'};open(data+'/imports/french.recall','w').write(json.dumps(d))
bridge.request(json.dumps({'action':'import'}));QTest.qWait(100)
r.setProperty('page','home');r.setProperty('notice','');QTest.qWait(100)
view.grabWindow().save('/tmp/recall-folder-style.png')
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(250,650));QTest.qWait(100)
assert r.property('currentFolder')=='Languages',r.property('currentFolder')
view.grabWindow().save(temp+'/recall-v2-folder.png')
assert r.property('error')=='',r.property('error')
# Select a deck, cancel confirmation, then confirm deletion using real clicks.
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(250,650));QTest.qWait(100)
assert r.property('currentFolder')=='Languages/French'
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(520,190));QTest.qWait(100)
assert r.property('selecting')
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(250,650));QTest.qWait(100)
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(300,1070));QTest.qWait(100)
assert r.property('page')=='delete'
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(170,1070));QTest.qWait(100)
assert r.property('page')=='home' and r.property('dueCount')==12
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(300,1070));QTest.qWait(100)
view.grabWindow().save('/tmp/recall-delete-confirm.png')
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(460,1070));QTest.qWait(100)
assert r.property('page')=='home' and not r.property('selecting')
assert r.property('dueCount')==6 and r.property('error')==''
state=json.loads(Path(data+'/state.json').read_text())['state']
assert all(c['deckId']!='french' for c in state['cards'])
# Finish all due cards, then practice through the home and completion buttons.
for c in state['cards']:
 bridge.request(json.dumps({'action':'review','id':c['id'],'expected':c['schedule']['reviews'],'rating':'easy'}))
QTest.qWait(100)
assert r.property('dueCount')==0
r.setProperty('currentFolder','');r.setProperty('notice','')
QMetaObject.invokeMethod(r,'buildEntries');QTest.qWait(100)
before_practice=Path(data+'/state.json').read_bytes()
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(300,465));QTest.qWait(100)
assert r.property('practice') and r.property('page')=='review'
for i in range(7):
 QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(300,980));QTest.qWait(50)
 assert r.property('revealed')
 if i==0: view.grabWindow().save('/tmp/recall-practice.png')
 QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(300,980));QTest.qWait(50)
 assert not r.property('revealed')
assert r.property('sessionCount')==7
assert Path(data+'/state.json').read_bytes()==before_practice
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(170,1070));QTest.qWait(100)
assert not r.property('practice') and r.property('page')=='home'
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(250,650));QTest.qWait(100)
assert r.property('page')=='review' and not r.property('practice')
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(300,980));QTest.qWait(100)
assert r.property('practice') and r.property('practiceTotal')==6
assert r.property('error')==''
# Holding a deck opens its menu without starting a review on release.
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(540,60));QTest.qWait(100)
r.setProperty('notice','')
QTest.mousePress(view,Qt.LeftButton,Qt.NoModifier,QPoint(250,650));QTest.qWait(750)
QTest.mouseRelease(view,Qt.LeftButton,Qt.NoModifier,QPoint(250,650));QTest.qWait(100)
assert r.property('page')=='home'
view.grabWindow().save('/tmp/recall-deck-menu.png')
def click_action(label):
 row=['Review','Practice','Move to folder','Select deck','Delete deck','Cancel'].index(label)
 QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(320,390+90*row));QTest.qWait(100)
click_action('Cancel')
assert r.property('page')=='home'
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(540,650));QTest.qWait(100)
click_action('Move to folder');assert r.property('page')=='folder'
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(540,60));QTest.qWait(100)
QTest.mousePress(view,Qt.LeftButton,Qt.NoModifier,QPoint(250,650));QTest.qWait(750)
QTest.mouseRelease(view,Qt.LeftButton,Qt.NoModifier,QPoint(250,650));QTest.qWait(100)
click_action('Delete deck');assert r.property('page')=='delete'
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(170,1070));QTest.qWait(100)
assert Path(data+'/state.json').read_bytes()==before_practice
# Folder menu deletes a parent and its nested deck only after confirmation.
QMetaObject.invokeMethod(r,'cancelSelection')
bridge.request(json.dumps({'action':'import'}))
# Re-upload the deleted French deck and create an empty descendant.
open(data+'/imports/french.recall','w').write(json.dumps(d))
bridge.request(json.dumps({'action':'import'}))
bridge.request(json.dumps({'action':'folder','folder':'Languages/Empty/Nested'}))
r.setProperty('notice','');QTest.qWait(100)
folder_before=Path(data+'/state.json').read_bytes()
QTest.mousePress(view,Qt.LeftButton,Qt.NoModifier,QPoint(250,650));QTest.qWait(750)
QTest.mouseRelease(view,Qt.LeftButton,Qt.NoModifier,QPoint(250,650));QTest.qWait(100)
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(320,615));QTest.qWait(100)
assert r.property('page')=='delete' and r.property('deletionFolder')=='Languages'
view.grabWindow().save('/tmp/recall-delete-folder.png')
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(170,1070));QTest.qWait(100)
assert Path(data+'/state.json').read_bytes()==folder_before
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(540,650));QTest.qWait(100)
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(320,615));QTest.qWait(100)
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(460,1070));QTest.qWait(100)
assert r.property('page')=='home' and r.property('error')==''
folder_state=json.loads(Path(data+'/state.json').read_text())['state']
assert len(folder_state['cards'])==6 and folder_state['folders']==[]
assert all(c['deckId']=='welcome' for c in folder_state['cards'])
print('Native backend + QML: load, reveal, rate, persist, undo, import, folder navigation, deletion, practice cycling and unchanged schedules, long-press menu and menu actions passed')
conn.send(struct.pack('<II',0xffffffff,0)); conn.close(); proc.wait(timeout=5)
server.close(); view.close(); shutil.rmtree(temp)
