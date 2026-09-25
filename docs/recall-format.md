# Paper Recall deck format, version 1

`.recall` is the app's own portable, UTF-8 JSON format. It is deliberately simple and documented so people and language models can generate decks. One file describes one deck. Use ordinary JSON, not Markdown fences, comments, trailing commas, or JavaScript. See [a complete example](../examples/french.recall).

| Field | Required | Meaning |
| --- | --- | --- |
| `format` | yes | Exactly `"paper-recall"` |
| `version` | yes | Integer `1` |
| `deck.id` | yes | Stable deck identifier |
| `deck.name` | yes | Display name, 1–200 UTF-8 bytes after trimming |
| `deck.folder` | no | Folder path, e.g. `Languages/French`; omitted or empty means top level |
| `cards` | yes | Array of 1–10,000 cards |
| `cards[].id` | yes | Stable identifier, unique within this file |
| `cards[].front` | yes | Question, 1–16,000 UTF-8 bytes after trimming |
| `cards[].back` | yes | Answer, 1–16,000 UTF-8 bytes after trimming |

IDs must match `^[a-zA-Z0-9][a-zA-Z0-9._-]{0,79}$`: 1–80 ASCII characters, starting with a letter or digit. Case matters. Display names and card text may use Unicode. Byte limits are not character limits. Text is rendered as plain text, with `\n` for newlines; HTML, Markdown, images, and executable content are not interpreted.

Folder paths use `/` separators, with at most six levels. Each trimmed segment is 1–80 UTF-8 bytes and cannot be `.` or `..`, contain backslashes, tabs, or line breaks, or be empty. Paths are logical labels, not filesystem paths.

Files must be at most 8 MiB. The total library is limited to 20,000 imported cards. Unknown properties, duplicate JSON object keys, duplicate card IDs, unsupported versions, malformed JSON, and nesting deeper than 32 levels are rejected. Do not include schedule fields. A failed file validation changes nothing for that file; a batch may still import other valid files.

## Updating a deck

The identity of a stored card is the pair `(deck.id, card.id)`. Retain both when correcting a question or answer: reimport updates text while preserving the schedule. Changing an ID creates a new card or deck. Import also updates the name and folder of the deck. Omitted cards are retained; imports never delete cards. A byte-for-byte previously imported file is skipped while its deck still exists, even if you have since edited the deck locally. Change the file to import a revised version.

Files awaiting import live in `/home/root/.local/share/paper-recall/imports/`. Successfully imported files are archived under `imported/`; failed files remain available for correction. Only regular `.recall` files in the inbox are read. The upload helper stages files with another extension to avoid partial imports.

## Export and validation

**Export** writes each deck to `exports/DECK_ID.recall` under the data directory. These files contain card content and folders, not scheduling, undo, or review counts. Full backups are separate; see [storage](storage.md).

Validate locally with `go run ./backend --validate examples/french.recall`, or on the tablet with `/home/root/xovi/exthome/appload/paper-recall/backend/entry --validate FILE`.
