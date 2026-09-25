#!/usr/bin/env python3
"""Build the frontend resource and dependency-free Linux ARM64 backend."""
import os
from pathlib import Path
import shutil
import subprocess
ROOT = Path(__file__).resolve().parents[1]
rcc = os.environ.get('RCC') or shutil.which('rcc') or shutil.which('rcc6')
if not rcc:
    try:
        import PySide6
        rcc = str(Path(PySide6.__file__).parent / 'Qt/libexec/rcc')
    except ImportError:
        raise SystemExit('Install Qt 6 rcc / PySide6-Essentials, or set RCC')
go = os.environ.get('GO') or shutil.which('go')
if not go: raise SystemExit('Install Go 1.23+ or set GO=/path/to/go')
out = ROOT / 'build/paper-recall'
(out / 'backend').mkdir(parents=True, exist_ok=True)
subprocess.run([rcc, '--binary', '--no-compress', str(ROOT / 'application.qrc'), '-o', str(out / 'resources.rcc')], check=True)
subprocess.run([go, 'build', '-trimpath', '-ldflags=-s -w', '-o', str(out / 'backend/entry'), './backend'], cwd=ROOT, env={**os.environ, 'CGO_ENABLED':'0', 'GOOS':'linux', 'GOARCH':'arm64'}, check=True)
for name in ('manifest.json', 'icon.png'): shutil.copy(ROOT / name, out / name)
shutil.copy(ROOT / 'backend/welcome.recall', out / 'backend/welcome.recall')
print(out)
