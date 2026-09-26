package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type Envelope struct {
	SHA256 string          `json:"sha256"`
	State  json.RawMessage `json:"state"`
}
type Store struct {
	Dir         string
	State       State
	Notice      string
	lock        *os.File
	writeFailed bool
}

func pack(s State) ([]byte, error) {
	b, e := json.Marshal(s)
	if e != nil {
		return nil, e
	}
	h := sha256.Sum256(b)
	return json.Marshal(Envelope{hex.EncodeToString(h[:]), b})
}
func unpack(b []byte) (State, error) {
	var env Envelope
	var s State
	if err := decodeStrict(b, &env); err != nil {
		return s, err
	}
	h := sha256.Sum256(env.State)
	if env.SHA256 != hex.EncodeToString(h[:]) {
		return s, errors.New("state checksum mismatch")
	}
	if err := decodeStrict(env.State, &s); err != nil {
		return s, err
	}
	return s, validateState(s)
}
func readState(path string) (State, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return State{}, e
	}
	return unpack(b)
}
func atomicWrite(path string, b []byte) error {
	dir := filepath.Dir(path)
	f, e := os.CreateTemp(dir, ".recall-write-*")
	if e != nil {
		return e
	}
	temp := f.Name()
	defer os.Remove(temp)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	if e = os.Rename(temp, path); e != nil {
		return e
	}
	d, e := os.Open(dir)
	if e != nil {
		return e
	}
	defer d.Close()
	return d.Sync()
}
func openStore(dir string) (*Store, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(filepath.Join(dir, "state.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, errors.New("another Paper Recall process is using this data")
	}
	s := &Store{Dir: dir, lock: f}
	a, ae := readState(filepath.Join(dir, "state.json"))
	b, be := readState(filepath.Join(dir, "state.backup.json"))
	switch {
	case ae == nil && (be != nil || a.Revision >= b.Revision):
		s.State = a
	case be == nil:
		s.State = b
		s.Notice = "Recovered progress from the backup. Your previous files have been preserved."
		if raw, e := os.ReadFile(filepath.Join(dir, "state.json")); e == nil {
			if e = atomicWrite(filepath.Join(dir, fmt.Sprintf("state.damaged-%d.json", b.Revision)), raw); e != nil {
				s.Close()
				return nil, e
			}
		}
		raw, _ := pack(b)
		if e = atomicWrite(filepath.Join(dir, "state.json"), raw); e != nil {
			s.Close()
			return nil, e
		}
	case os.IsNotExist(ae) && os.IsNotExist(be):
		s.State = initialState()
	default:
		s.Close()
		return nil, fmt.Errorf("cannot read progress or backup; files were preserved. Primary: %v; backup: %v", ae, be)
	}
	return s, nil
}
func (s *Store) Close() {
	if s.lock != nil {
		syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)
		s.lock.Close()
		s.lock = nil
	}
}
func (s *Store) Save(next State) (saveErr error) {
	if s.lock == nil {
		return errors.New("storage is closed")
	}
	if s.writeFailed {
		return errors.New("a write failed; close and reopen Paper Recall before saving again")
	}
	next.Version = 1
	next.Revision = s.State.Revision + 1
	if e := validateState(next); e != nil {
		return e
	}
	raw, e := pack(next)
	if e != nil {
		return e
	}
	old, e := pack(s.State)
	if e != nil {
		return e
	}
	defer func() {
		if saveErr != nil {
			s.writeFailed = true
		}
	}()
	if e = atomicWrite(filepath.Join(s.Dir, "state.backup.json"), old); e != nil {
		return fmt.Errorf("backup failed: %w", e)
	}
	if e = atomicWrite(filepath.Join(s.Dir, "state.json"), raw); e != nil {
		return fmt.Errorf("save failed: %w", e)
	}
	s.State = cloneState(next)
	return nil
}
