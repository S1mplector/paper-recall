package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type Card struct {
	ID       string   `json:"id"`
	DeckID   string   `json:"deckId"`
	Deck     string   `json:"deck"`
	Folder   string   `json:"folder"`
	Front    string   `json:"front"`
	Back     string   `json:"back"`
	Schedule Schedule `json:"schedule"`
}
type Undo struct {
	CardID   string   `json:"cardId"`
	Before   Schedule `json:"before"`
	Day      string   `json:"day"`
	ActionID string   `json:"actionId"`
}
type State struct {
	Version      int             `json:"version"`
	Revision     int64           `json:"revision"`
	Cards        []Card          `json:"cards"`
	Folders      []string        `json:"folders"`
	Daily        map[string]int  `json:"daily"`
	Imported     map[string]bool `json:"imported"`
	Undo         *Undo           `json:"undo,omitempty"`
	LastMutation int64           `json:"lastMutation"`
}
type DeckFile struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	Deck    struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Folder string `json:"folder,omitempty"`
	} `json:"deck"`
	Cards []struct {
		ID    string `json:"id"`
		Front string `json:"front"`
		Back  string `json:"back"`
	} `json:"cards"`
}

var validID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,79}$`)

func decodeStrict(raw []byte, dst any) error {
	if err := checkJSONKeys(json.NewDecoder(bytes.NewReader(raw)), 0); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		return errors.New("expected exactly one JSON object")
	}
	return nil
}
func parseDeck(raw []byte) (DeckFile, error) {
	var d DeckFile
	if len(raw) > 8*1024*1024 {
		return d, errors.New("deck exceeds 8 MiB")
	}
	if err := decodeStrict(raw, &d); err != nil {
		return d, err
	}
	if d.Format != "paper-recall" || d.Version != 1 {
		return d, errors.New("expected format paper-recall and version 1")
	}
	if !validID.MatchString(d.Deck.ID) {
		return d, errors.New("deck.id must use 1–80 letters, numbers, dots, underscores or hyphens")
	}
	var folderErr error
	d.Deck.Folder, folderErr = normalizeFolder(d.Deck.Folder)
	if folderErr != nil {
		return d, folderErr
	}
	d.Deck.Name = strings.TrimSpace(d.Deck.Name)
	if d.Deck.Name == "" || len(d.Deck.Name) > 200 {
		return d, errors.New("deck.name must contain 1–200 UTF-8 bytes")
	}
	if len(d.Cards) == 0 || len(d.Cards) > 10000 {
		return d, errors.New("a deck must contain 1–10000 cards")
	}
	seen := map[string]bool{}
	for i := range d.Cards {
		c := &d.Cards[i]
		if !validID.MatchString(c.ID) || seen[c.ID] {
			return d, fmt.Errorf("card %d: invalid or duplicate id", i+1)
		}
		seen[c.ID] = true
		c.Front = strings.TrimSpace(c.Front)
		c.Back = strings.TrimSpace(c.Back)
		if c.Front == "" || c.Back == "" || len(c.Front) > 16000 || len(c.Back) > 16000 {
			return d, fmt.Errorf("card %s: front and back must each contain 1–16000 UTF-8 bytes", c.ID)
		}
	}
	return d, nil
}
func initialState() State {
	return State{Version: 1, Cards: []Card{}, Daily: map[string]int{}, Imported: map[string]bool{}}
}
func cloneState(s State) State { b, _ := json.Marshal(s); var n State; json.Unmarshal(b, &n); return n }
func validateState(s State) error {
	if s.Version != 1 || s.Revision < 0 || s.Cards == nil || s.Daily == nil || s.Imported == nil {
		return errors.New("invalid state structure")
	}
	ids := map[string]bool{}
	for _, c := range s.Cards {
		if c.ID == "" || ids[c.ID] || c.Front == "" || c.Back == "" {
			return errors.New("invalid or duplicate stored card")
		}
		ids[c.ID] = true
		p := c.Schedule
		if p.Due < 0 || p.Interval < 0 || p.Interval > maxInterval || p.Ease < 0 || p.Ease > 3 || p.Reviews < 0 || p.Lapses < 0 || p.LastReview < 0 {
			return errors.New("invalid stored schedule")
		}
		if p.Phase != "" && p.Phase != "new" && p.Phase != "learning" && p.Phase != "relearning" && p.Phase != "review" {
			return errors.New("unknown stored phase")
		}
	}
	return nil
}
func mergeDeck(s *State, d DeckFile) (added, updated int) {
	index := map[string]int{}
	for i, c := range s.Cards {
		index[c.ID] = i
	}
	for i := range s.Cards {
		if s.Cards[i].DeckID == d.Deck.ID {
			s.Cards[i].Deck = d.Deck.Name
			s.Cards[i].Folder = d.Deck.Folder
		}
	}
	for _, c := range d.Cards {
		id := d.Deck.ID + "/" + c.ID
		if i, ok := index[id]; ok {
			s.Cards[i].Front = c.Front
			s.Cards[i].Back = c.Back
			updated++
		} else {
			s.Cards = append(s.Cards, Card{ID: id, DeckID: d.Deck.ID, Deck: d.Deck.Name, Folder: d.Deck.Folder, Front: c.Front, Back: c.Back})
			added++
		}
	}
	return
}

func normalizeFolder(folder string) (string, error) {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return "", nil
	}
	parts := strings.Split(folder, "/")
	if len(parts) > 6 {
		return "", errors.New("folders support at most six levels")
	}
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "." || p == ".." || len(p) > 80 || strings.ContainsAny(p, "\\\n\r\t") {
			return "", errors.New("invalid folder path")
		}
		parts[i] = p
	}
	return strings.Join(parts, "/"), nil
}

// Reject duplicate object keys instead of silently accepting the last value.
func checkJSONKeys(d *json.Decoder, depth int) error {
	if depth > 32 {
		return errors.New("JSON nesting exceeds 32 levels")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delim == '{' {
		seen := map[string]bool{}
		for d.More() {
			key, e := d.Token()
			if e != nil {
				return e
			}
			name, ok := key.(string)
			if !ok {
				return errors.New("invalid JSON key")
			}
			if seen[name] {
				return fmt.Errorf("duplicate JSON key %q", name)
			}
			seen[name] = true
			if e = checkJSONKeys(d, depth+1); e != nil {
				return e
			}
		}
	} else if delim == '[' {
		for d.More() {
			if e := checkJSONKeys(d, depth+1); e != nil {
				return e
			}
		}
	} else {
		return errors.New("unexpected JSON delimiter")
	}
	_, err = d.Token()
	return err
}
