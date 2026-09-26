package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

const maxMessage = 196608 // Fits worst-case escaped card text and Linux seqpacket limits.

func send(conn *net.UnixConn, kind uint32, b []byte) error {
	h := make([]byte, 8)
	binary.LittleEndian.PutUint32(h, kind)
	binary.LittleEndian.PutUint32(h[4:], uint32(len(b)))
	if _, e := conn.Write(h); e != nil {
		return e
	}
	if len(b) > 0 {
		_, e := conn.Write(b)
		return e
	}
	return nil
}
func sendView(conn *net.UnixConn, v View) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	// Small packets stay below Linux's Unix seqpacket limit, including long cards.
	if e = send(conn, 100, []byte("begin")); e != nil {
		return e
	}
	for len(b) > 0 {
		n := min(48000, len(b)) // Do not split a UTF-8 code point.
		for n < len(b) && n > 0 && (b[n]&0xc0) == 0x80 {
			n--
		}
		if e = send(conn, 101, b[:n]); e != nil {
			return e
		}
		b = b[n:]
	}
	return send(conn, 102, []byte("end"))
}
func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: paper-recall SOCKET | --validate FILE | --check-state DIR")
	}
	if os.Args[1] == "--validate" {
		if len(os.Args) != 3 {
			return fmt.Errorf("specify a .recall file")
		}
		b, e := os.ReadFile(os.Args[2])
		if e != nil {
			return e
		}
		d, e := parseDeck(b)
		if e != nil {
			return e
		}
		fmt.Printf("Valid deck: %s (%d cards)\n", d.Deck.Name, len(d.Cards))
		return nil
	}
	dir := os.Getenv("PAPER_RECALL_DATA")
	if dir == "" {
		dir = "/home/root/.local/share/paper-recall"
	}
	if os.Args[1] == "--check-state" {
		if len(os.Args) != 3 {
			return fmt.Errorf("specify a data directory")
		}
		s, e := openStore(os.Args[2])
		if e != nil {
			return e
		}
		defer s.Close()
		fmt.Printf("Valid state: revision %d, %d cards. %s\n", s.State.Revision, len(s.State.Cards), s.Notice)
		return nil
	}
	conn, e := net.DialUnix("unixpacket", nil, &net.UnixAddr{Name: os.Args[1], Net: "unixpacket"})
	if e != nil {
		return e
	}
	defer conn.Close()
	store, startErr := openStore(dir)
	if store != nil {
		defer store.Close()
	}
	if startErr == nil && store.State.Revision == 0 && len(store.State.Cards) == 0 {
		exe, _ := os.Executable()
		raw, err := os.ReadFile(filepath.Join(filepath.Dir(exe), "welcome.recall"))
		if err == nil {
			_, _, err = store.importBytes(raw, time.Now().UnixMilli())
		}
		if err != nil {
			startErr = fmt.Errorf("could not load welcome deck: %w", err)
		}
	}
	// Do not replace old-format progress silently. Legacy migration is explicit.
	buffer := make([]byte, maxMessage)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			return err
		}
		if n != 8 {
			return fmt.Errorf("invalid AppLoad header")
		}
		kind := binary.LittleEndian.Uint32(buffer)
		length := binary.LittleEndian.Uint32(buffer[4:])
		if length > maxMessage {
			return fmt.Errorf("request too large")
		}
		var body []byte
		if length > 0 {
			n, err = conn.Read(buffer)
			if err != nil {
				return err
			}
			if uint32(n) != length {
				return fmt.Errorf("truncated request")
			}
			body = buffer[:n]
		}
		if kind == 0xffffffff {
			return nil
		}
		if kind != 1 {
			continue
		}
		var req Request
		if err = decodeStrict(body, &req); err != nil {
			if e = sendView(conn, View{Error: "Invalid request: " + err.Error()}); e != nil {
				return e
			}
			continue
		}
		var view View
		if startErr != nil {
			view = View{Token: req.Token, Action: req.Action, Error: "Storage error: " + startErr.Error()}
		} else {
			view = store.handle(req, time.Now().UnixMilli())
		}
		if e = sendView(conn, view); e != nil {
			return e
		}
	}
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
