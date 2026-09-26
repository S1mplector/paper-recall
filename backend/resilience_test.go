package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplayAfterUndoAndRestart(t *testing.T) {
	s := testStore(t)
	now := int64(1800000000000)
	s.importBytes([]byte(testDeck), now)
	req := Request{Action: "review", ID: "french/hello", Rating: "easy", Token: "original-rating"}
	s.handle(req, now)
	s.handle(Request{Action: "undo", Token: "undo-rating"}, now)
	dir := s.Dir
	s.Close()
	r, e := openStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	revision := r.State.Revision
	r.handle(req, now)
	if r.State.Revision != revision || r.State.Cards[0].Schedule.Reviews != 0 {
		t.Fatal("delayed rating replayed after undo")
	}
}
func TestMovedDeckDoesNotReuseIdentity(t *testing.T) {
	s := testStore(t)
	now := int64(1800000000000)
	s.handle(Request{Action: "save", Token: "a", DeckName: "Basics", Front: "Q", Back: "A"}, now)
	id := s.State.Cards[0].DeckID
	s.handle(Request{Action: "moveDeck", Deck: id, Folder: "Archive"}, now)
	s.handle(Request{Action: "save", Token: "b", DeckName: "Basics", Front: "Q2", Back: "A2"}, now)
	if len(s.State.Cards) != 2 || s.State.Cards[0].DeckID == s.State.Cards[1].DeckID {
		t.Fatal("moved deck reused identity")
	}
}
func TestStrictMalformedInputs(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(strings.Replace(testDeck, `"version":1`, `"version":1,"Version":2`, 1)),
		[]byte(strings.Replace(testDeck, "Hello", string([]byte{0xff}), 1)),
	} {
		if _, e := parseDeck(raw); e == nil {
			t.Fatal("accepted ambiguous or invalid encoding")
		}
	}
}
func TestStoredMetadataValidation(t *testing.T) {
	s := testStore(t)
	s.importBytes([]byte(testDeck), 1800000000000)
	cases := []func(*State){
		func(s *State) { s.Cards[0].DeckID = "../../escape" },
		func(s *State) { s.Cards[0].ID = "wrong/card" },
		func(s *State) { s.Cards[0].Schedule.Ease = math.NaN() },
		func(s *State) { s.Cards[0].Schedule.Step = -1 },
		func(s *State) { s.Folders = []string{"../escape"} },
		func(s *State) { s.Daily["2026-09-26"] = -1 },
		func(s *State) { s.Undo = &Undo{CardID: "missing", Before: Schedule{}} },
	}
	for i, change := range cases {
		n := cloneState(s.State)
		change(&n)
		if validateState(n) == nil {
			t.Fatalf("accepted corrupt metadata case %d", i)
		}
	}
}
func TestClosedStoreCannotWrite(t *testing.T) {
	s := testStore(t)
	s.Close()
	if e := s.Save(initialState()); e == nil {
		t.Fatal("write after unlocking allowed")
	}
}
func TestPrimaryWriteFailureAndRestart(t *testing.T) {
	s := testStore(t)
	s.importBytes([]byte(testDeck), 1800000000000)
	path := filepath.Join(s.Dir, "state.json")
	os.Remove(path)
	os.Mkdir(path, 0700)
	before, _ := json.Marshal(s.State)
	v := s.handle(Request{Action: "review", ID: "french/hello", Rating: "good"}, 1800000000000)
	after, _ := json.Marshal(s.State)
	if v.Error == "" || string(before) != string(after) {
		t.Fatal("failed primary write advanced memory")
	}
	os.Remove(path)
	if e := s.Save(cloneState(s.State)); e == nil {
		t.Fatal("write failure did not latch")
	}
	dir := s.Dir
	s.Close()
	r, e := openStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	if len(r.State.Cards) != 1 || r.State.Cards[0].Schedule.Reviews != 0 {
		t.Fatal("backup failed to preserve unacknowledged rating")
	}
}
func FuzzDeckParsing(f *testing.F) {
	f.Add([]byte(testDeck))
	f.Add([]byte(`{"format":"paper-recall"}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, b []byte) {
		d, e := parseDeck(b)
		if e != nil {
			return
		}
		s := initialState()
		mergeDeck(&s, d)
		if e = validateState(s); e != nil {
			t.Fatal(e)
		}
		raw, e := json.Marshal(d)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = parseDeck(raw); e != nil {
			t.Fatal(e)
		}
	})
}
func FuzzSchedulerSequence(f *testing.F) {
	f.Add([]byte{0, 1, 2, 2, 3, 0, 2})
	f.Add([]byte{3, 3, 3, 3})
	f.Fuzz(func(t *testing.T, ratings []byte) {
		if len(ratings) > 500 {
			ratings = ratings[:500]
		}
		p := Schedule{}
		now := int64(1800000000000)
		for _, r := range ratings {
			n, e := schedule(p, []string{"again", "hard", "good", "easy"}[r%4], now)
			if e != nil {
				t.Fatal(e)
			}
			if n.Due <= now || n.Reviews != p.Reviews+1 || n.Interval > maxInterval || n.Ease < 1.3 || n.Ease > 3 {
				t.Fatalf("invalid transition %+v -> %+v", p, n)
			}
			p = n
			now = n.Due
		}
	})
}

func TestUndoShowsRestoredCardAndRespectsDeck(t *testing.T) {
	s := testStore(t)
	now := int64(1800000000000)
	s.importBytes([]byte(testDeck), now)
	s.importBytes([]byte(strings.ReplaceAll(testDeck, "french", "spanish")), now)
	s.handle(Request{Action: "review", ID: "spanish/hello", Rating: "easy"}, now)
	if s.view(Request{Deck: "french"}, now).CanUndo {
		t.Fatal("undo offered for another deck")
	}
	if s.handle(Request{Action: "undo", Deck: "french"}, now).Error == "" {
		t.Fatal("undid another deck")
	}
	v := s.handle(Request{Action: "undo"}, now)
	if v.Error != "" || v.Current == nil || v.Current.ID != "spanish/hello" {
		t.Fatal("undo displayed wrong card")
	}
}
func TestSaveCopiesStateAndValidationDoesNotLatch(t *testing.T) {
	s := testStore(t)
	s.importBytes([]byte(testDeck), 1800000000000)
	n := cloneState(s.State)
	n.Cards[0].Back = ""
	if e := s.Save(n); e == nil {
		t.Fatal("invalid save succeeded")
	}
	n = cloneState(s.State)
	if e := s.Save(n); e != nil {
		t.Fatal("validation failure poisoned storage", e)
	}
	n.Cards[0].Back = "mutated without save"
	if s.State.Cards[0].Back == n.Cards[0].Back {
		t.Fatal("caller owns committed slice")
	}
}
func TestRecoveredPrimaryIsPreserved(t *testing.T) {
	s := testStore(t)
	s.importBytes([]byte(testDeck), 1800000000000)
	s.handle(Request{Action: "folder", Folder: "Empty"}, 1800000000000)
	dir := s.Dir
	s.Close()
	bad := []byte("truncated state")
	os.WriteFile(filepath.Join(dir, "state.json"), bad, 0600)
	r, e := openStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	paths, _ := filepath.Glob(filepath.Join(dir, "state.damaged-*.json"))
	if len(paths) != 1 {
		t.Fatal("damaged evidence missing")
	}
	b, _ := os.ReadFile(paths[0])
	if string(b) != string(bad) {
		t.Fatal("damaged evidence altered")
	}
}
func TestSchedulerRejectsUnsafeValues(t *testing.T) {
	for _, p := range []Schedule{{Ease: math.NaN()}, {Interval: math.Inf(1)}, {Step: -1}, {Due: -1}, {Reviews: 1000000001}} {
		if _, e := schedule(p, "good", 1800000000000); e == nil {
			t.Fatal("unsafe schedule accepted")
		}
	}
	for _, now := range []int64{-1, math.MaxInt64} {
		if _, e := schedule(Schedule{}, "easy", now); e == nil {
			t.Fatal("unsafe time accepted")
		}
	}
}
