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
