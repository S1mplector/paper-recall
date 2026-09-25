# Storage and recovery

The app lives at `/home/root/xovi/exthome/appload/paper-recall/`. Data lives separately at `/home/root/.local/share/paper-recall/`. `PAPER_RECALL_DATA` overrides the data directory for development.

- `state.json`: current library, schedules, folders, recent daily counts, import hashes, and one undo record.
- `state.backup.json`: the previous successfully saved revision.
- `state.lock`: advisory exclusive lock; a second backend cannot write concurrently.
- `imports/`, `imported/`, `exports/`: incoming, archived, and exported deck files.

State is a JSON envelope with a SHA-256 checksum of its embedded state. Saving validates the new state, writes and syncs a temporary backup, renames it and syncs the directory, then does the same for the new primary. The in-memory state advances only after success. A failed write stops further writes until reopening, avoiding ambiguous retries. The checksum detects accidental damage; it is not encryption or authentication.

At startup the app validates both files and selects the newest valid revision. If it recovers from the backup, it preserves a readable damaged primary under `state.damaged-REVISION.json`, repairs the primary, and displays a notice. Recovery may lose the last change because the backup is one revision behind. If both files are damaged, the app reports an error and preserves them instead of silently creating a new library. No software can guarantee recovery from total storage failure: keep an external backup.

## Back up

Close Paper Recall first, then copy the entire data directory to your computer:

```sh
mkdir -p private
scp -O -r root@10.11.99.1:/home/root/.local/share/paper-recall private/tablet-backup
```

Keep dated copies. The repository ignores `private/`. Do not commit personal decks, device passwords, or progress files.

## Restore

Close Paper Recall. Preserve a copy of the current directory before replacing anything. Restore `state.json` and `state.backup.json` together from the same backup; the newer valid revision wins. Set owner root and file permissions 600. Validate while the app is closed:

```sh
ssh root@10.11.99.1 '/home/root/xovi/exthome/appload/paper-recall/backend/entry --check-state /home/root/.local/share/paper-recall'
```

This check takes the storage lock and can repair a primary from a valid backup. Reopen the app afterwards. A lock error means a backend is still running; do not delete the state files to fix it.

The initial prototype used Qt Settings in `state.ini`. The owner's existing progress was migrated during setup, and its original `state.pre-native.ini` was retained on-device. This release does not automatically migrate arbitrary legacy INI files. Keep those files until you have verified your current library.

## App updates

The installer archives the preceding application build as `app-backup-TIMESTAMP` under the data directory. It preserves user data. To roll back the application, stop the tablet interface, restore the archived application directory under AppLoad, and start the interface again; do not replace the data directory with an application archive.
