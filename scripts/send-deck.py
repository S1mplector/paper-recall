#!/usr/bin/env python3
"""Send a .recall file over SSH, validate on the tablet, then expose it for import."""
import argparse
from pathlib import Path
import subprocess
import hashlib
p=argparse.ArgumentParser(description=__doc__)
p.add_argument('deck',type=Path);p.add_argument('--host',default='root@10.11.99.1');a=p.parse_args()
raw=a.deck.read_bytes()
if len(raw)>8*1024*1024:p.error('Deck exceeds 8 MiB')
if a.host.startswith('-') or any(c.isspace() for c in a.host):p.error('Invalid SSH host')
identity=hashlib.sha256(raw).hexdigest()[:20]
staging=f'/home/root/.local/share/paper-recall/imports/{identity}.upload'
subprocess.run(['ssh',a.host,'mkdir -p /home/root/.local/share/paper-recall/imports'],check=True)
subprocess.run(['scp','-O',str(a.deck.resolve()),a.host+':'+staging],check=True)
# Paths contain only fixed text and a digest, never a user-supplied shell fragment.
cmd=f'/home/root/xovi/exthome/appload/paper-recall/backend/entry --validate {staging} && mv {staging} {staging[:-7]}.recall'
subprocess.run(['ssh',a.host,cmd],check=True)
print('Deck uploaded and validated. Tap Import in Paper Recall.')
