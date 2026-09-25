package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testDeck = `{"format":"paper-recall","version":1,"deck":{"id":"french","name":"French","folder":"Languages/French"},"cards":[{"id":"hello","front":"Bonjour","back":"Hello"}]}`

func testStore(t *testing.T) *Store {
	t.Helper()
	s, e := openStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	return s
}
func TestImportReviewRestartUndo(t *testing.T) {
	s := testStore(t)
	now := int64(1800000000000)
	if _, _, e := s.importBytes([]byte(testDeck), now); e != nil {
		t.Fatal(e)
	}
	v := s.handle(Request{Action: "review", ID: "french/hello", Rating: "easy", Expected: 0, Token: "review-1"}, now)
	if v.Error != "" || v.DueCount != 0 || v.ReviewedToday != 1 {
		t.Fatalf("%+v", v)
	}
	duplicate := s.handle(Request{Action: "review", ID: "french/hello", Rating: "easy", Expected: 0, Token: "review-1"}, now)
	if duplicate.Error == "" || duplicate.ReviewedToday != 1 {
		t.Fatalf("duplicate: %+v", duplicate)
	}
	// Reimport edits content, preserves the interval, and never deletes cards.
	changed := strings.Replace(testDeck, "Hello", "Hi", 1)
	if _, _, e := s.importBytes([]byte(changed), now); e != nil {
		t.Fatal(e)
	}
	if s.State.Cards[0].Back != "Hi" || s.State.Cards[0].Schedule.Interval != 4 {
		t.Fatal(s.State.Cards)
	}
	dir := s.Dir
	s.Close()
	s, e := openStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if s.State.Cards[0].Schedule.Reviews != 1 {
		t.Fatal("lost progress on restart")
	}
	v = s.handle(Request{Action: "undo"}, now)
	if v.Error != "" || v.DueCount != 1 || v.ReviewedToday != 0 || s.State.Cards[0].Back != "Hi" {
		t.Fatal(v)
	}
}
func TestBackupRecoveryAndCorruptionProtection(t *testing.T) {
	s := testStore(t)
	s.importBytes([]byte(testDeck), 1800000000000)
	s.handle(Request{Action: "folder", Folder: "Books"}, 1800000000000)
	dir := s.Dir
	s.Close()
	os.WriteFile(filepath.Join(dir, "state.json"), []byte("broken"), 0600)
	r, e := openStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.State.Cards) != 1 || r.Notice == "" {
		t.Fatal("backup not recovered")
	}
	r.Close()
	os.WriteFile(filepath.Join(dir, "state.json"), []byte("broken"), 0600)
	os.WriteFile(filepath.Join(dir, "state.backup.json"), []byte("broken too"), 0600)
	if r, e = openStore(dir); e == nil {
		r.Close()
		t.Fatal("silently reset damaged data")
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "state.json"))
	if string(raw) != "broken" {
		t.Fatal("overwrote damaged file")
	}
}
func TestFailedSaveDoesNotAdvanceMemory(t *testing.T) {
	s := testStore(t)
	s.importBytes([]byte(testDeck), 1800000000000)
	os.Remove(filepath.Join(s.Dir, "state.backup.json"))
	os.Mkdir(filepath.Join(s.Dir, "state.backup.json"), 0700)
	v := s.handle(Request{Action: "review", ID: "french/hello", Rating: "easy", Expected: 0}, 1800000000000)
	if v.Error == "" || s.State.Cards[0].Schedule.Reviews != 0 {
		t.Fatal("failed save advanced the card")
	}
}
func TestExclusiveLock(t *testing.T) {
	s := testStore(t)
	if other, e := openStore(s.Dir); e == nil {
		other.Close()
		t.Fatal("second writer allowed")
	}
}
func TestImportValidationAndFolders(t *testing.T) {
	bad := []string{strings.Replace(testDeck, `"version":1`, `"version":2`, 1), strings.Replace(testDeck, `"hello"`, `"../escape"`, 1), strings.Replace(testDeck, `"back":"Hello"`, `"back":""`, 1), strings.Replace(testDeck, `"name":"French"`, `"name":"French","unknown":true`, 1), testDeck + "{}"}
	for _, b := range bad {
		if _, e := parseDeck([]byte(b)); e == nil {
			t.Fatal("accepted", b)
		}
	}
	s := testStore(t)
	s.importBytes([]byte(testDeck), 1800000000000)
	v := s.handle(Request{Action: "moveDeck", Deck: "french", Folder: "Study / Languages"}, 1800000000000)
	if v.Error != "" || v.Decks[0].Folder != "Study/Languages" {
		t.Fatal(v)
	}
	v = s.handle(Request{Action: "folder", Folder: "../invalid"}, 1800000000000)
	if v.Error == "" {
		t.Fatal("bad folder accepted")
	}
	if e := s.exportDecks(); e != nil {
		t.Fatal(e)
	}
	raw, _ := os.ReadFile(filepath.Join(s.Dir, "exports/french.recall"))
	var d DeckFile
	json.Unmarshal(raw, &d)
	if d.Deck.Folder != "Study/Languages" {
		t.Fatal(d)
	}
}
func TestClockRollback(t *testing.T) {
	s := testStore(t)
	s.importBytes([]byte(testDeck), 1800000000000)
	v := s.handle(Request{Action: "review", ID: "french/hello", Rating: "good"}, 1800000000000-10*minute)
	if v.Error == "" || !v.ClockWarning {
		t.Fatal(v)
	}
}
func TestMalformedImportIsAtomicAndRetrySafe(t *testing.T) {
	s := testStore(t)
	s.importBytes([]byte(testDeck), 1800000000000)
	revision := s.State.Revision
	a, u, e := s.importBytes([]byte(testDeck), 1800000000000)
	if e != nil || a != 0 || u != 0 || s.State.Revision != revision {
		t.Fatal("identical import changed progress")
	}
	bad := strings.Replace(testDeck, `"back":"Hello"`, `"back":""`, 1)
	if _, _, e = s.importBytes([]byte(bad), 1800000000000); e == nil || s.State.Revision != revision {
		t.Fatal("invalid import changed data")
	}
}

func TestDuplicateJSONKeysRejected(t *testing.T) {
	bad := strings.Replace(testDeck, `"version":1`, `"version":1,"version":1`, 1)
	if _, e := parseDeck([]byte(bad)); e == nil {
		t.Fatal("accepted duplicate key")
	}
}

func TestSameDeckNameInDifferentFolders(t *testing.T) {
	s := testStore(t)
	for i, folder := range []string{"Languages/French", "Languages/Spanish", "Languages/French"} {
		v := s.handle(Request{Action: "save", Token: []string{"a", "b", "c"}[i], DeckName: "Basics", Folder: folder, Front: "Question", Back: "Answer"}, 1800000000000)
		if v.Error != "" {
			t.Fatal(v.Error)
		}
	}
	cards := s.State.Cards
	if len(cards) != 3 || cards[0].DeckID == cards[1].DeckID || cards[0].DeckID != cards[2].DeckID {
		t.Fatalf("incorrect deck grouping: %+v", cards)
	}
}

func TestDeleteDecksPersistenceAndReimport(t *testing.T) {
	s := testStore(t)
	now := int64(1800000000000)
	s.importBytes([]byte(testDeck), now)
	other := strings.ReplaceAll(testDeck, "french", "spanish")
	s.importBytes([]byte(other), now)
	s.handle(Request{Action: "review", ID: "french/hello", Rating: "good", Token: "review"}, now)
	before := s.State.Revision
	if v := s.handle(Request{Action: "deleteDecks", DeckIDs: []string{"french", "missing"}}, now); v.Error == "" || s.State.Revision != before {
		t.Fatal("partial deletion of invalid selection")
	}
	if v := s.handle(Request{Action: "deleteDecks"}, now); v.Error == "" {
		t.Fatal("accepted empty selection")
	}
	v := s.handle(Request{Action: "deleteDecks", DeckIDs: []string{"french"}}, now)
	if v.Error != "" || len(s.State.Cards) != 1 || s.State.Cards[0].DeckID != "spanish" || v.CanUndo || v.ReviewedToday != 1 {
		t.Fatalf("bad deletion: %+v", v)
	}
	dir := s.Dir
	s.Close()
	reopened, e := openStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if len(reopened.State.Cards) != 1 {
		t.Fatal("deletion did not persist")
	}
	if _, _, e = reopened.importBytes([]byte(testDeck), now); e != nil {
		t.Fatal(e)
	}
	if len(reopened.State.Cards) != 2 {
		t.Fatal("could not reimport deleted deck")
	}
	v = reopened.handle(Request{Action: "deleteDecks", DeckIDs: []string{"french", "spanish"}}, now)
	if v.Error != "" || len(reopened.State.Cards) != 0 {
		t.Fatal("bulk deletion failed")
	}
	reopened.Close()
	empty, e := openStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer empty.Close()
	if empty.State.Revision == 0 || len(empty.State.Cards) != 0 {
		t.Fatal("empty library not preserved")
	}
}

func TestFailedDeletionPreservesLibrary(t *testing.T) {
	s := testStore(t)
	s.importBytes([]byte(testDeck), 1800000000000)
	s.Dir = filepath.Join(s.Dir, "missing")
	v := s.handle(Request{Action: "deleteDecks", DeckIDs: []string{"french"}}, 1800000000000)
	if v.Error == "" || len(s.State.Cards) != 1 {
		t.Fatal("failed save deleted cards")
	}
}

func TestPracticeCyclesWithoutChangingProgress(t *testing.T) {
	s := testStore(t)
	now := int64(1800000000000)
	s.importBytes([]byte(testDeck), now)
	s.importBytes([]byte(strings.ReplaceAll(testDeck, "french", "spanish")), now)
	for _, id := range []string{"french/hello", "spanish/hello"} {
		s.handle(Request{Action: "review", ID: id, Rating: "easy"}, now)
	}
	before, _ := json.Marshal(s.State)
	for i := 0; i < 5; i++ {
		v := s.handle(Request{Action: "practiceNext", Practice: true, PracticeIndex: i}, now)
		if v.Error != "" || v.Current == nil || v.DueCount != 0 || v.PracticeTotal != 2 || v.Current.ID != s.State.Cards[i%2].ID {
			t.Fatalf("practice failed: %+v", v)
		}
	}
	v := s.handle(Request{Action: "status", Practice: true, Deck: "spanish", PracticeIndex: 7}, now)
	if v.Current == nil || v.Current.DeckID != "spanish" || v.PracticeTotal != 1 {
		t.Fatal("practice ignored deck")
	}
	if v = s.handle(Request{Action: "status", Practice: true, Deck: "missing"}, now); v.Current != nil {
		t.Fatal("practice returned unrelated card")
	}
	if v = s.handle(Request{Action: "review", Practice: true, ID: "french/hello", Rating: "again", Expected: 1}, now); v.Error == "" {
		t.Fatal("allowed practice rating")
	}
	after, _ := json.Marshal(s.State)
	if string(before) != string(after) {
		t.Fatal("practice modified saved progress")
	}
}

func TestDeleteFolderIncludesDescendantsOnly(t *testing.T) {
	s := testStore(t)
	now := int64(1800000000000)
	s.importBytes([]byte(testDeck), now)
	s.importBytes([]byte(strings.ReplaceAll(strings.ReplaceAll(testDeck, "french", "other"), "Languages/French", "LanguagesOther")), now)
	s.handle(Request{Action: "folder", Folder: "Languages/Empty/Nested"}, now)
	s.handle(Request{Action: "review", ID: "french/hello", Rating: "good"}, now)
	before := s.State.Revision
	for _, path := range []string{"", "/", "Missing"} {
		if v := s.handle(Request{Action: "deleteFolder", Folder: path}, now); v.Error == "" || s.State.Revision != before {
			t.Fatal("invalid delete modified state")
		}
	}
	v := s.handle(Request{Action: "deleteFolder", Folder: "Languages"}, now)
	if v.Error != "" || len(s.State.Cards) != 1 || s.State.Cards[0].Folder != "LanguagesOther" || len(s.State.Folders) != 0 || v.CanUndo || v.ReviewedToday != 1 {
		t.Fatalf("bad folder deletion: %+v", v)
	}
	dir := s.Dir
	s.Close()
	reopened, e := openStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if len(reopened.State.Cards) != 1 || len(reopened.State.Folders) != 0 {
		t.Fatal("folder deletion did not persist")
	}
	reopened.handle(Request{Action: "folder", Folder: "Empty"}, now)
	if v = reopened.handle(Request{Action: "deleteFolder", Folder: "Empty"}, now); v.Error != "" || len(reopened.State.Folders) != 0 {
		t.Fatal("empty folder deletion failed")
	}
	reopened.Dir = filepath.Join(dir, "missing")
	if v = reopened.handle(Request{Action: "deleteFolder", Folder: "LanguagesOther"}, now); v.Error == "" || len(reopened.State.Cards) != 1 {
		t.Fatal("failed deletion lost cards")
	}
}
