# Resilience audit — 26 September 2026

## Fixed issues

- A delayed rating could be replayed after Undo restored its previous review count. The last 256 mutation tokens are now persisted and rejected on replay, including after restart. The frontend also ignores duplicate replies after accepting the original response. Expected review counts remain an additional stale-rating guard.
- Moving a locally created deck and creating another with its old name and location could reuse the original deck ID. New deck IDs now incorporate the creation token.
- Saved metadata validation did not sufficiently check deck/card identity, consistent deck placement, folder paths, undo targets, counters, or non-finite schedule values. Invalid state is rejected so a valid backup can be used instead.
- Invalid UTF-8 and case-variant duplicate JSON keys could be interpreted ambiguously. Both are rejected.
- A closed store could write without its lock. Saves now require the store to remain open. Committed state is copied to prevent mutation through caller-owned slices. Invalid candidate state does not poison the writer; actual filesystem write failures still latch it closed until reopening.
- Undo could reveal a different due card instead of the restored card. It now returns the restored card and is scoped to the selected deck.
- Long, JSON-escaped card text could exceed the request buffer despite being within text limits. The buffer now accommodates worst-case escaping. Response chunks are tested for intact multibyte characters.
- Folder names matching JavaScript prototype properties could disrupt folder rendering. Folder maps now have no prototype.
- Transport timeout previously permitted further ambiguous operations. Requests stop until reopening. Changing cards resets answer visibility and scrolling; navigation is disabled while a request is pending.
- Anki conversion removed lines beginning with # even within multiline answers. Only leading export headers are removed now; names, card lengths, counts, and file size are checked. Generated deck IDs include a name digest to distinguish names with the same ASCII slug.
- Exports now validate every deck against the import format before replacing output files. Oversized merged decks report an error instead of silently producing unimportable output.
- Installation validates existing data using the staged binary and restores the previous application directory if the final staged-directory move fails.

## Verification

- Go regression tests with the race detector: passed; backend statement coverage 72.1% (the separate UI/CLI runs are not included in this coverage figure).
- Go static checks (`go vet`): passed.
- Parser fuzzing: 114,603 executions, 15-second requested budget; no failure.
- Scheduler-sequence fuzzing: 585,365 executions, 15-second budget; no failure. Each sequence checks up to 500 successive ratings and scheduling bounds.
- Python importer tests: 6 passed.
- QML + real native backend integration: review, undo, import, folders, bulk/folder deletion, cancellation, practice, long press, duplicate-response handling, and prototype-like folder labels.
- Protocol tests: chunked Unicode responses and maximum escaped card requests.
- Recovery tests: invalid primary, both files damaged, exclusive locking, failed backup write, failed primary write, failed-write latching, reopening, and preservation of damaged input.

Run the checks with the README commands. Fuzz targets are `FuzzDeckParsing` and `FuzzSchedulerSequence`.

## Practical limits

Tests cannot prove absence of bugs or guarantee recovery from a physical storage failure. The backup holds the preceding revision, so recovery can lose the newest change. Save operations use file and directory fsync, but actual power-cut testing on the tablet was not performed. Replay detection retains 256 mutation tokens rather than unlimited history. Keep independent backups of the data directory.

Older builds do not understand the newly stored `recentActions` field. If rolling back to a pre-audit build, also restore its matching data snapshot; never let an older binary repair a newer data file. The pre-update primary and backup were copied privately before installation.
