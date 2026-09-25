# Paper Recall

An offline flashcard app for reMarkable Paper Pro Move, launched from **sidebar → AppLoad → Paper Recall**. Built with a Qt Quick interface and a dependency-free Go backend. Verified on the owner's Move running firmware 3.28; other models and firmware versions have not been tested.

- Tap to reveal; rate Again, Hard, Good, or Easy with the next interval shown.
- Learning, review, and relearning steps; undo the last rating.
- Create and edit plain-text cards, organize decks in nested folders, and move decks.
- Import GPT-generated `.recall` decks without resetting existing schedules.
- Checksummed state, atomic saves, a previous-revision backup, and automatic backup recovery.

## Using the app

Open a deck or choose **Start reviewing** for all decks. Reveal the answer, then rate how well you remembered it. **Close** returns to the tablet interface. **+ Folder** accepts paths such as `Languages/French`; **Press and hold a deck**, or tap its **•••** button, to open Review, Practice, Move to folder, Select deck, and Delete deck options. Deleting still asks for confirmation. Folders have a tabbed shape and a folder icon; tap one to browse it. **Move to folder** changes a deck's folder. Leave the destination empty to move a deck to the top level. **Create a card** creates a card in the current folder. The editor scrolls to accommodate long text and the keyboard.

## Practice when nothing is due

Tap **Practice cards** on the home screen when you are caught up, or **Practice again** at the end of a deck's review session. Reveal each answer and tap **Next card**. Practice cycles through the selected deck (or all decks) for as long as you like. **Finish practice** returns home. Practice does not change due dates, review totals, or the last rating's undo record.

## Delete decks

Tap **Select**, then tap the decks you want to remove. Browse folders to select decks in different folders. Tap **Delete selected**, review the deck names and card count, and confirm **Delete decks**. Cancel leaves the library unchanged. Deletion removes those cards and their schedules; folders and historical daily review totals remain. It cannot be undone with Undo rating. Export first if you want to keep card content. Re-uploading a deleted deck imports it again with fresh progress.

## Delete folders

Press and hold a folder, or tap its **•••** button, then choose **Delete folder**. Review the listed decks and card count before confirming. This removes the folder, all nested folders, and their decks and schedules. Empty folders can also be deleted. Other folders and decks are preserved. Cancel makes no changes; deletion cannot be undone in the app.

## Make and import decks

See the [format specification](docs/recall-format.md), [ready-to-copy GPT prompt](docs/gpt-deck-prompt.md), and [example deck](examples/french.recall).

```sh
python3 scripts/send-deck.py examples/french.recall
```

Connect the tablet over USB with developer-mode SSH enabled. The default address is `root@10.11.99.1`; pass `--host root@ADDRESS` if needed. SSH asks for your device credentials; no credentials are stored in this repository. After upload, tap **Import** in Paper Recall. Uploads are validated on the tablet before being made available to the importer.

To convert a basic two-column Anki text export:

```sh
python3 scripts/import_anki.py cards.txt --deck French --output private/french.recall
python3 scripts/send-deck.py private/french.recall
```

The converter reads tab-separated question/answer columns, supports quoted multiline fields and Anki's `#html:true` header, and strips HTML. It does not import `.apkg`, media, cloze behavior, templates, or Anki scheduling. Its card IDs derive from content: changing text in a new conversion creates a new ID. For iterative updates, edit the `.recall` file while retaining its IDs.

## Build and install

Requires Python 3, Go 1.23+, and Qt 6 `rcc` (or `PySide6-Essentials`). The tablet must already have [XOVI](https://github.com/asivery/xovi) and [AppLoad](https://github.com/asivery/rm-appload) installed for its firmware. The installer checks for AppLoad; it does not install or upgrade these system extensions.

```sh
python3 scripts/build.py
sh scripts/install.sh root@10.11.99.1
```

Set `GO=/path/to/go` or `RCC=/path/to/rcc` as needed. The build produces `build/paper-recall/` with an ARM64 backend. Installation stages and validates the build, restarts the tablet interface, and keeps the prior application under the data directory. Close your notebook and Paper Recall before installing. User data is kept separately and is not replaced.

## Checks

```sh
go test ./backend
go vet ./backend
python3 -m unittest discover -s tests
# Optional Qt integration check (requires PySide6 and a completed build):
python3 tests/ui_smoke.py
```

The UI check uses an isolated temporary data directory and the real backend protocol. See [scheduling](docs/scheduling.md) and [storage and recovery](docs/storage.md) for behavior and limits.

## Scope

This is a standalone app, with no cloud account, telemetry, network service, or Anki synchronization. Cards are plain text. Deck exports contain content and folder placement, not review progress; back up the data directory to preserve progress. Third-party launchers may need an update after a reMarkable firmware upgrade.
