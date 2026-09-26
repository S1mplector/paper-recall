package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestChunkedUnicodeTransport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "socket")
	listener, e := net.ListenUnix("unixpacket", &net.UnixAddr{Name: path, Net: "unixpacket"})
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	done := make(chan error, 1)
	want := View{Token: "test", Notice: strings.Repeat("🧠é世界", 16000)}
	go func() {
		c, e := listener.AcceptUnix()
		if e == nil {
			defer c.Close()
			e = sendView(c, want)
		}
		done <- e
	}()
	c, e := net.DialUnix("unixpacket", nil, &net.UnixAddr{Name: path, Net: "unixpacket"})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	var joined bytes.Buffer
	chunks := 0
	for {
		h := make([]byte, 8)
		n, e := c.Read(h)
		if e != nil || n != 8 {
			t.Fatalf("header %d %v", n, e)
		}
		kind := binary.LittleEndian.Uint32(h)
		length := binary.LittleEndian.Uint32(h[4:])
		b := make([]byte, length)
		n, e = c.Read(b)
		if e != nil || n != int(length) {
			t.Fatalf("body %d %v", n, e)
		}
		if !utf8.Valid(b) {
			t.Fatal("split UTF-8 code point")
		}
		if kind == 101 {
			joined.Write(b)
			chunks++
		}
		if kind == 102 {
			break
		}
	}
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	var got View
	if e = json.Unmarshal(joined.Bytes(), &got); e != nil {
		t.Fatal(e)
	}
	if got.Notice != want.Notice || chunks < 2 {
		t.Fatal("chunked response changed content")
	}
}
func TestMaximumEscapedCardRequestFits(t *testing.T) {
	b, e := json.Marshal(Request{Action: "save", Token: "unique", Front: strings.Repeat("\x01", 16000), Back: strings.Repeat("\x02", 16000), DeckName: strings.Repeat("n", 200), Folder: strings.Repeat("f", 80)})
	if e != nil || len(b) > maxMessage {
		t.Fatalf("valid input too large for transport: %d %v", len(b), e)
	}
}
