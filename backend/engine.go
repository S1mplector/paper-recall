package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Request struct {
	DeckIDs  []string `json:"deckIds"`
	Action   string   `json:"action"`
	Token    string   `json:"token"`
	Deck     string   `json:"deck"`
	ID       string   `json:"id"`
	Rating   string   `json:"rating"`
	Expected int      `json:"expected"`
	Front    string   `json:"front"`
	Back     string   `json:"back"`
	DeckName string   `json:"deckName"`
	Folder   string   `json:"folder"`
}
type DeckView struct {
	Folder string `json:"folder"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Total  int    `json:"total"`
	Due    int    `json:"due"`
}
type CardView struct {
	Card
	Labels map[string]string `json:"labels"`
}
type View struct {
	Token         string     `json:"token"`
	Action        string     `json:"action"`
	Error         string     `json:"error"`
	Notice        string     `json:"notice"`
	Decks         []DeckView `json:"decks"`
	Folders       []string   `json:"folders"`
	Current       *CardView  `json:"current"`
	DueCount      int        `json:"dueCount"`
	ReviewedToday int        `json:"reviewedToday"`
	CanUndo       bool       `json:"canUndo"`
	NextDue       int64      `json:"nextDue"`
	ClockWarning  bool       `json:"clockWarning"`
}

func (s *Store) view(req Request, now int64) View {
	v := View{Token: req.Token, Action: req.Action, Notice: s.Notice, Decks: []DeckView{}, Folders: s.State.Folders, CanUndo: s.State.Undo != nil}
	v.ReviewedToday = s.State.Daily[time.UnixMilli(now).Format("2006-01-02")]
	v.ClockWarning = now < s.State.LastMutation-5*minute
	groups := map[string]*DeckView{}
	due := []Card{}
	for _, c := range s.State.Cards {
		d := groups[c.DeckID]
		if d == nil {
			d = &DeckView{ID: c.DeckID, Name: c.Deck, Folder: c.Folder}
			groups[c.DeckID] = d
		}
		d.Total++
		if c.Schedule.Due <= now {
			d.Due++
			v.DueCount++
		}
		if req.Deck == "" || c.DeckID == req.Deck {
			if c.Schedule.Due <= now {
				due = append(due, c)
			} else if v.NextDue == 0 || c.Schedule.Due < v.NextDue {
				v.NextDue = c.Schedule.Due
			}
		}
	}
	// Due learning/relearning cards first, then due reviews, then new cards.
	priority := func(c Card) int {
		if c.Schedule.Phase == "learning" || c.Schedule.Phase == "relearning" {
			return 0
		}
		if c.Schedule.Phase == "review" {
			return 1
		}
		return 2
	}
	sort.SliceStable(due, func(i, j int) bool {
		a, b := priority(due[i]), priority(due[j])
		if a != b {
			return a < b
		}
		return due[i].Schedule.Due < due[j].Schedule.Due
	})
	if len(due) > 0 {
		v.Current = &CardView{Card: due[0], Labels: map[string]string{}}
		for _, r := range []string{"again", "hard", "good", "easy"} {
			p, _ := schedule(due[0].Schedule, r, now)
			v.Current.Labels[r] = intervalLabel(p.Due - now)
		}
	}
	for _, d := range groups {
		v.Decks = append(v.Decks, *d)
	}
	sort.Slice(v.Decks, func(i, j int) bool { return v.Decks[i].Name < v.Decks[j].Name })
	return v
}
func (s *Store) handle(req Request, now int64) View {
	var err error
	next := cloneState(s.State)
	mutate := false
	switch req.Action {
	case "status":
	case "review":
		if now < s.State.LastMutation-5*minute {
			err = fmt.Errorf("tablet clock moved backwards; correct its date and time before reviewing")
			break
		}
		i := -1
		for j, c := range next.Cards {
			if c.ID == req.ID {
				i = j
				break
			}
		}
		if i < 0 {
			err = fmt.Errorf("card not found")
			break
		}
		c := &next.Cards[i]
		if c.Schedule.Reviews != req.Expected {
			err = fmt.Errorf("this card has already changed; refresh and try again")
			break
		}
		if c.Schedule.Due > now {
			err = fmt.Errorf("this card is not due yet")
			break
		}
		previous := c.Schedule
		var p Schedule
		p, err = schedule(previous, req.Rating, now)
		if err != nil {
			break
		}
		today := time.UnixMilli(now).Format("2006-01-02")
		next.Undo = &Undo{CardID: c.ID, Before: previous, Day: today, ActionID: req.Token}
		c.Schedule = p
		next.Daily[today]++
		cutoff := time.UnixMilli(now).AddDate(-2, 0, 0).Format("2006-01-02")
		for d := range next.Daily {
			if d < cutoff {
				delete(next.Daily, d)
			}
		}
		mutate = true
	case "undo":
		if next.Undo == nil {
			err = fmt.Errorf("no rating to undo")
			break
		}
		found := false
		for i := range next.Cards {
			if next.Cards[i].ID == next.Undo.CardID {
				next.Cards[i].Schedule = next.Undo.Before
				found = true
				break
			}
		}
		if !found {
			err = fmt.Errorf("cannot find the reviewed card")
			break
		}
		next.Daily[next.Undo.Day] = max(0, next.Daily[next.Undo.Day]-1)
		next.Undo = nil
		mutate = true
	case "deleteDecks":
		if len(req.DeckIDs) == 0 {
			err = fmt.Errorf("select at least one deck")
			break
		}
		selected := map[string]bool{}
		existing := map[string]bool{}
		for _, c := range next.Cards {
			existing[c.DeckID] = true
		}
		for _, id := range req.DeckIDs {
			if !existing[id] {
				err = fmt.Errorf("a selected deck no longer exists; refresh and select again")
				break
			}
			selected[id] = true
		}
		if err != nil {
			break
		}
		kept := make([]Card, 0, len(next.Cards))
		for _, c := range next.Cards {
			if selected[c.DeckID] {
				if next.Undo != nil && next.Undo.CardID == c.ID {
					next.Undo = nil
				}
			} else {
				kept = append(kept, c)
			}
		}
		next.Cards = kept
		mutate = true
	case "folder", "moveDeck":
		var folder string
		folder, err = normalizeFolder(req.Folder)
		if err != nil {
			break
		}
		if req.Action == "folder" && folder == "" {
			err = fmt.Errorf("enter a folder name")
			break
		}
		if req.Action == "moveDeck" {
			found := false
			for i := range next.Cards {
				if next.Cards[i].DeckID == req.Deck {
					next.Cards[i].Folder = folder
					found = true
				}
			}
			if !found {
				err = fmt.Errorf("deck not found")
				break
			}
		}
		if folder != "" {
			exists := false
			for _, f := range next.Folders {
				if f == folder {
					exists = true
				}
			}
			if !exists {
				next.Folders = append(next.Folders, folder)
			}
		}
		mutate = true
	case "save":
		var folderErr error
		req.Folder, folderErr = normalizeFolder(req.Folder)
		if folderErr != nil {
			err = folderErr
			break
		}
		req.Front = strings.TrimSpace(req.Front)
		req.Back = strings.TrimSpace(req.Back)
		req.DeckName = strings.TrimSpace(req.DeckName)
		if req.Front == "" || req.Back == "" || req.DeckName == "" || len(req.Front) > 16000 || len(req.Back) > 16000 || len(req.DeckName) > 200 {
			err = fmt.Errorf("enter a deck name, question and answer (maximum 16000 bytes per side)")
			break
		}
		if req.ID != "" {
			found := false
			for i := range next.Cards {
				c := &next.Cards[i]
				if c.ID == req.ID {
					c.Front = req.Front
					c.Back = req.Back
					found = true
					break
				}
			}
			if !found {
				err = fmt.Errorf("card not found")
				break
			}
		} else {
			deckID := ""
			for _, c := range next.Cards {
				if c.Deck == req.DeckName && c.Folder == req.Folder {
					deckID = c.DeckID
					break
				}
			}
			if deckID == "" {
				h := sha256.Sum256([]byte(req.Folder + "\x00" + req.DeckName))
				deckID = "local-" + hex.EncodeToString(h[:8])
			}
			h := sha256.Sum256([]byte(req.Token))
			id := deckID + "/card-" + hex.EncodeToString(h[:12])
			exists := false
			for _, c := range next.Cards {
				if c.ID == id {
					exists = true
				}
			}
			if !exists {
				next.Cards = append(next.Cards, Card{ID: id, DeckID: deckID, Deck: req.DeckName, Folder: req.Folder, Front: req.Front, Back: req.Back})
			}
		}
		mutate = true
	case "import":
		err = s.importInbox(now)
	case "export":
		err = s.exportDecks()
	default:
		err = fmt.Errorf("unknown action")
	}
	if mutate && err == nil {
		next.LastMutation = max(now, next.LastMutation)
		err = s.Save(next)
	}
	v := s.view(req, now)
	if err != nil {
		v.Error = err.Error()
	}
	return v
}
func (s *Store) importBytes(raw []byte, now int64) (int, int, error) {
	d, e := parseDeck(raw)
	if e != nil {
		return 0, 0, e
	}
	h := sha256.Sum256(raw)
	hash := hex.EncodeToString(h[:])
	if s.State.Imported[hash] {
		// A deliberately re-uploaded deck can be restored after deletion.
		for _, c := range s.State.Cards {
			if c.DeckID == d.Deck.ID {
				return 0, 0, nil
			}
		}
	}
	next := cloneState(s.State)
	a, u := mergeDeck(&next, d)
	if len(next.Cards) > 20000 {
		return 0, 0, fmt.Errorf("library limit is 20000 cards")
	}
	next.Imported[hash] = true
	next.LastMutation = max(now, next.LastMutation)
	return a, u, s.Save(next)
}
func (s *Store) importInbox(now int64) error {
	dir := filepath.Join(s.Dir, "imports")
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	paths, e := filepath.Glob(filepath.Join(dir, "*.recall"))
	if e != nil {
		return e
	}
	if len(paths) == 0 {
		s.Notice = "No .recall files found in the imports folder."
		return nil
	}
	added, updated := 0, 0
	failures := []string{}
	for _, p := range paths {
		info, e := os.Lstat(p)
		if e != nil || !info.Mode().IsRegular() || info.Size() > 8*1024*1024 {
			failures = append(failures, filepath.Base(p)+": not a regular file or exceeds 8 MiB")
			continue
		}
		raw, e := os.ReadFile(p)
		if e != nil {
			failures = append(failures, e.Error())
			continue
		}
		a, u, e := s.importBytes(raw, now)
		if e != nil {
			failures = append(failures, filepath.Base(p)+": "+e.Error())
			continue
		}
		added += a
		updated += u
		// Imported-file hashes make retry safe even if archiving is interrupted.
		archive := filepath.Join(s.Dir, "imported")
		if e = os.MkdirAll(archive, 0700); e == nil {
			h := sha256.Sum256(raw)
			e = os.Rename(p, filepath.Join(archive, hex.EncodeToString(h[:8])+"-"+filepath.Base(p)))
		}
		if e != nil {
			failures = append(failures, "Imported, but could not archive "+filepath.Base(p))
		}
	}
	s.Notice = fmt.Sprintf("Imported %d new cards; updated %d. Existing progress was preserved.", added, updated)
	if len(failures) > 0 {
		return fmt.Errorf("%s\n%s", s.Notice, strings.Join(failures, "\n"))
	}
	return nil
}
func (s *Store) exportDecks() error {
	dir := filepath.Join(s.Dir, "exports")
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	decks := map[string]*DeckFile{}
	for _, c := range s.State.Cards {
		d := decks[c.DeckID]
		if d == nil {
			d = &DeckFile{Format: "paper-recall", Version: 1}
			d.Deck.ID = c.DeckID
			d.Deck.Name = c.Deck
			d.Deck.Folder = c.Folder
			decks[c.DeckID] = d
		}
		id := strings.TrimPrefix(c.ID, c.DeckID+"/")
		d.Cards = append(d.Cards, struct {
			ID    string `json:"id"`
			Front string `json:"front"`
			Back  string `json:"back"`
		}{id, c.Front, c.Back})
	}
	for id, d := range decks {
		raw, e := json.MarshalIndent(d, "", "  ")
		if e != nil {
			return e
		}
		if e = atomicWrite(filepath.Join(dir, id+".recall"), raw); e != nil {
			return e
		}
	}
	s.Notice = fmt.Sprintf("Exported %d decks to the exports folder. Full progress is in state.json and its backup.", len(decks))
	return nil
}
