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
view.grabWindow().save(temp+'/recall-v2-folders.png')
QTest.mouseClick(view,Qt.LeftButton,Qt.NoModifier,QPoint(250,650));QTest.qWait(100)
assert r.property('currentFolder')=='Languages',r.property('currentFolder')
view.grabWindow().save(temp+'/recall-v2-folder.png')
assert r.property('error')=='',r.property('error')
print('Native backend + QML: load, reveal, rate, persist, undo, import, folder navigation passed')
conn.send(struct.pack('<II',0xffffffff,0)); conn.close(); proc.wait(timeout=5)
server.close(); view.close(); shutil.rmtree(temp)
