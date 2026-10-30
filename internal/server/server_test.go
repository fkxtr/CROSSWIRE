package server

import (
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fkxtr/CROSSWIRE/internal/protocol"
	"github.com/gorilla/websocket"
)

func setup(t *testing.T, d time.Duration) (*Server, string) {
	t.Helper()
	s := New(Config{Duration: d})
	h := httptest.NewServer(s)
	t.Cleanup(func() { s.Close(); h.Close() })
	return s, "ws" + strings.TrimPrefix(h.URL, "http") + "/ws"
}
func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}
func send(t *testing.T, c *websocket.Conn, v any) {
	t.Helper()
	c.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := c.WriteJSON(v); err != nil {
		t.Fatal(err)
	}
}
func read(t *testing.T, c *websocket.Conn, kind string) protocol.Message {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		var m protocol.Message
		if err := c.ReadJSON(&m); err != nil {
			t.Fatal(err)
		}
		if m.Type == kind {
			return m
		}
	}
}
func pair(t *testing.T, url string) (*websocket.Conn, *websocket.Conn, protocol.Message, protocol.Message) {
	t.Helper()
	a := dial(t, url)
	send(t, a, map[string]any{"type": "join"})
	read(t, a, "waiting")
	b := dial(t, url)
	send(t, b, map[string]any{"type": "join"})
	return a, b, read(t, a, "matched"), read(t, b, "matched")
}
func draw(x, y int) map[string]any { return map[string]any{"type": "draw", "x": x, "y": y} }
func erase(x, y int) map[string]any {
	return map[string]any{"type": "erase", "x": x, "y": y}
}
func eventually(t *testing.T, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !f() {
		if time.Now().After(deadline) {
			t.Fatal("cleanup timeout")
		}
		runtime.Gosched()
	}
}
func TestMatchmakingAndIsolation(t *testing.T) {
	s, url := setup(t, time.Minute)
	a, b, ma, mb := pair(t, url)
	if ma.Room != mb.Room || ma.Player != 1 || mb.Player != 2 || ma.Seconds != 60 {
		t.Fatalf("bad match: %+v %+v", ma, mb)
	}
	c := dial(t, url)
	send(t, c, map[string]any{"type": "join"})
	read(t, c, "waiting")
	send(t, a, draw(3, 4))
	ua, ub := read(t, a, "canvas_update"), read(t, b, "canvas_update")
	if *ua.Canvas != *ub.Canvas || ua.Canvas[4][3] != 1 {
		t.Fatal("canvas diverged")
	}
	send(t, b, draw(3, 4))
	ua, ub = read(t, a, "canvas_update"), read(t, b, "canvas_update")
	if *ua.Canvas != *ub.Canvas || ua.Canvas[4][3] != 2 {
		t.Fatal("last writer did not win")
	}
	s.mu.Lock()
	if len(s.rooms) != 1 || s.waiting == nil {
		t.Error("third player entered active room")
	}
	s.mu.Unlock()
	d := dial(t, url)
	send(t, d, map[string]any{"type": "join"})
	mc, md := read(t, c, "matched"), read(t, d, "matched")
	if mc.Room == ma.Room || mc.Room != md.Room || *mc.Canvas != (protocol.Canvas{}) {
		t.Fatal("rooms not independent")
	}
}
func TestInvalidMessages(t *testing.T) {
	_, url := setup(t, time.Minute)
	a, b, _, _ := pair(t, url)
	invalid := []string{`{`, `null`, `{"type":"draw","x":-1,"y":0}`, `{"type":"draw","x":40,"y":0}`, `{"type":"draw","x":0,"y":16}`, `{"type":"draw","x":0}`, `{"type":"draw","x":0.5,"y":1}`, `{"type":"erase","x":0}`, `{"type":"reset","x":0,"y":0}`, `{"type":"draw","x":0,"y":0,"text":"\u001b[2J"}`, `{"type":"join"}`, `{"type":"draw","x":0,"y":0} {}`, `{"type":"unknown"}`}
	for _, data := range invalid {
		a.SetWriteDeadline(time.Now().Add(time.Second))
		if err := a.WriteMessage(websocket.TextMessage, []byte(data)); err != nil {
			t.Fatal(err)
		}
		read(t, a, "error")
	}
	send(t, a, draw(1, 1))
	ua, ub := read(t, a, "canvas_update"), read(t, b, "canvas_update")
	expected := protocol.Canvas{}
	expected[1][1] = 1
	if *ua.Canvas != expected || *ub.Canvas != expected {
		t.Fatal("invalid input changed canvas")
	}
}
func TestEraseAndReset(t *testing.T) {
	_, url := setup(t, time.Minute)
	a, b, _, _ := pair(t, url)
	send(t, a, draw(5, 6))
	read(t, a, "canvas_update")
	read(t, b, "canvas_update")
	send(t, b, erase(5, 6))
	ua, ub := read(t, a, "canvas_update"), read(t, b, "canvas_update")
	if *ua.Canvas != *ub.Canvas || ua.Canvas[6][5] != 0 {
		t.Fatal("erase did not synchronize")
	}
	send(t, a, draw(1, 1))
	read(t, a, "canvas_update")
	read(t, b, "canvas_update")
	send(t, b, map[string]any{"type": "reset"})
	ua, ub = read(t, a, "canvas_reset"), read(t, b, "canvas_reset")
	if *ua.Canvas != (protocol.Canvas{}) || *ub.Canvas != (protocol.Canvas{}) {
		t.Fatal("reset did not clear both canvases")
	}
}
func TestConcurrentDrawing(t *testing.T) {
	_, url := setup(t, time.Minute)
	a, b, _, _ := pair(t, url)
	var wg sync.WaitGroup
	for i, c := range []*websocket.Conn{a, b} {
		wg.Add(1)
		go func(i int, c *websocket.Conn) {
			defer wg.Done()
			for x := 0; x < 20; x++ {
				if err := c.WriteJSON(draw(x, i)); err != nil {
					t.Error(err)
					return
				}
			}
		}(i, c)
	}
	wg.Wait()
	var ua, ub protocol.Message
	for i := 0; i < 40; i++ {
		ua = read(t, a, "canvas_update")
		ub = read(t, b, "canvas_update")
		if *ua.Canvas != *ub.Canvas {
			t.Fatal("event ordering differs")
		}
	}
	for x := 0; x < 20; x++ {
		if ua.Canvas[0][x] != 1 || ua.Canvas[1][x] != 2 {
			t.Fatal("lost concurrent event")
		}
	}
}
func TestExpirationAndCleanup(t *testing.T) {
	s, url := setup(t, 120*time.Millisecond)
	a, b, _, _ := pair(t, url)
	send(t, a, draw(0, 0))
	fa, fb := read(t, a, "finished"), read(t, b, "finished")
	if fa.Seconds != 0 || *fa.Canvas != *fb.Canvas || fa.Canvas[0][0] != 1 {
		t.Fatal("invalid final state")
	}
	eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.rooms) == 0 })
	send(t, a, draw(0, 1))
	read(t, a, "error")
}
func TestDisconnection(t *testing.T) {
	s, url := setup(t, time.Minute)
	a, b, _, _ := pair(t, url)
	a.Close()
	read(t, b, "player_left")
	send(t, b, draw(0, 0))
	if read(t, b, "canvas_update").Canvas[0][0] != 2 {
		t.Fatal("remaining player cannot finish")
	}
	b.Close()
	eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.rooms) == 0 && len(s.peers) == 0 })
	c := dial(t, url)
	send(t, c, map[string]any{"type": "join"})
	read(t, c, "waiting")
	c.Close()
	eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.waiting == nil })
	pair(t, url) // New clients can reconnect and match after disconnects.
}
func TestBoundsAndRateLimits(t *testing.T) {
	_, url := setup(t, time.Minute)
	c := dial(t, url)
	send(t, c, map[string]any{"type": "draw", "x": 0, "y": 0})
	read(t, c, "error")
	if err := c.WriteMessage(websocket.TextMessage, []byte(strings.Repeat("x", 513))); err != nil {
		t.Fatal(err)
	}
	c.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("oversized frame accepted")
	}
	c = dial(t, url)
	send(t, c, map[string]any{"type": "join"})
	read(t, c, "waiting")
	for i := 0; i < 110; i++ {
		if c.WriteJSON(map[string]any{"type": "bad"}) != nil {
			break
		}
	}
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	closed := false
	for i := 0; i < 120; i++ {
		if _, _, err := c.ReadMessage(); err != nil {
			if e, ok := err.(interface{ Timeout() bool }); ok && e.Timeout() {
				t.Fatal("rate limit did not close connection")
			}
			closed = true
			break
		}
	}
	if !closed {
		t.Fatal("rate limit absent")
	}
}
func TestCloseActiveRooms(t *testing.T) {
	s, url := setup(t, time.Minute)
	pair(t, url)
	s.Close()
	s.Close()
}
