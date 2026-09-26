#!/bin/sh
set -eu
host=${1:-root@10.11.99.1}
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
test -f "$root/build/paper-recall/backend/entry" || { echo 'Run scripts/build.py first'; exit 1; }
# Stage on the same filesystem; the running version remains intact until upload finishes.
ssh "$host" 'test -f /home/root/xovi/extensions.d/appload.so && mkdir -p /home/root/.local/share/paper-recall/imports /home/root/xovi/exthome/appload/.paper-recall-stage/backend'
scp -O "$root/build/paper-recall/manifest.json" "$root/build/paper-recall/icon.png" "$root/build/paper-recall/resources.rcc" "$host:/home/root/xovi/exthome/appload/.paper-recall-stage/"
scp -O "$root/build/paper-recall/backend/entry" "$root/build/paper-recall/backend/welcome.recall" "$host:/home/root/xovi/exthome/appload/.paper-recall-stage/backend/"
ssh "$host" 'set -e
base=/home/root/xovi/exthome/appload
chmod 700 "$base/.paper-recall-stage/backend/entry"
"$base/.paper-recall-stage/backend/entry" --validate "$base/.paper-recall-stage/backend/welcome.recall"
systemctl stop xochitl
old="/home/root/.local/share/paper-recall/app-backup-$(date +%s)-$$"
recover() {
    result=$?
    if [ ! -d "$base/paper-recall" ] && [ -d "$old" ]; then mv "$old" "$base/paper-recall"; fi
    systemctl start xochitl
    exit "$result"
}
trap recover EXIT
"$base/.paper-recall-stage/backend/entry" --check-state /home/root/.local/share/paper-recall
# Previous application builds are archived outside the launcher directory.
if [ -d "$base/paper-recall" ]; then mv "$base/paper-recall" "$old"; fi
mv "$base/.paper-recall-stage" "$base/paper-recall"
systemctl start xochitl
trap - EXIT'
echo 'Installed. Open the sidebar > AppLoad > Paper Recall.'
