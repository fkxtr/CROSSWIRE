package client

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"github.com/fkxtr/CROSSWIRE/internal/protocol"
	"github.com/fkxtr/CROSSWIRE/internal/server"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func key(m Model, k rune) Model { n, _ := m.Update(tea.KeyPressMsg{Code: k}); return n.(Model) }
func TestTerminalStates(t *testing.T) {
	tr := &Transport{outgoing: make(chan protocol.Input, 64), done: make(chan struct{})}
	m := New(tr, false)
	if !strings.Contains(m.View().Content, "too small") {
		t.Fatal("missing resize guidance")
	}
	n, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = n.(Model)
	if !strings.Contains(m.View().Content, "Searching") {
		t.Fatal("missing waiting state")
	}
	c := protocol.Canvas{}
	n, _ = m.Update(protocol.Message{Type: "matched", Player: 2, Canvas: &c, Seconds: 60})
	m = n.(Model)
	m = key(m, 'd')
	m = key(m, 's')
	m = key(m, ' ')
	event := <-tr.outgoing
	if *event.X != 1 || *event.Y != 1 || m.player != 2 {
		t.Fatal("controls or color broken")
	}
	for i := 0; i < 100; i++ {
		m = key(m, 'a')
		m = key(m, 'w')
	}
	if m.x != 0 || m.y != 0 {
		t.Fatal("cursor escaped canvas")
	}
	c[2][3] = 1
	n, _ = m.Update(protocol.Message{Type: "canvas_update", Canvas: &c, Seconds: 22})
	m = n.(Model)
	if m.canvas[2][3] != 1 || !strings.Contains(m.View().Content, "00:22") {
		t.Fatal("update not rendered")
	}
	n, _ = m.Update(protocol.Message{Type: "player_left"})
	m = n.(Model)
	if !strings.Contains(m.status, "left") {
		t.Fatal("no disconnect notice")
	}
	n, _ = m.Update(protocol.Message{Type: "finished", Canvas: &c})
	m = n.(Model)
	m = key(m, 'd')
	m = key(m, ' ')
	if m.state != "finished" || m.x != 0 || len(tr.outgoing) != 0 {
		t.Fatal("finished round still interactive")
	}
	n, _ = m.Update(failure{})
	if n.(Model).state != "finished" {
		t.Fatal("lost final canvas after disconnect")
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'q'}); cmd == nil {
		t.Fatal("quit ignored")
	}
	n, _ = New(tr, false).Update(failure{})
	if n.(Model).state != "disconnected" {
		t.Fatal("connection error hidden")
	}
}
func TestTransportEndToEnd(t *testing.T) {
	s := server.New(server.Config{Duration: 150 * time.Millisecond})
	h := httptest.NewServer(s)
	defer h.Close()
	defer s.Close()
	url := "ws" + strings.TrimPrefix(h.URL, "http") + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	a, err := Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	next := func(tr *Transport, kind string) protocol.Message {
		t.Helper()
		for {
			select {
			case v := <-tr.incoming:
				if m, ok := v.(protocol.Message); ok && m.Type == kind {
					return m
				}
			case <-ctx.Done():
				t.Fatal("transport timeout")
				return protocol.Message{}
			}
		}
	}
	ma, mb := next(a, "matched"), next(b, "matched")
	if ma.Room != mb.Room {
		t.Fatal("not matched")
	}
	a.draw(2, 3)
	ua, ub := next(a, "canvas_update"), next(b, "canvas_update")
	if *ua.Canvas != *ub.Canvas || ua.Canvas[3][2] != 1 {
		t.Fatal("transport desynchronized")
	}
	fa, fb := next(a, "finished"), next(b, "finished")
	if *fa.Canvas != *fb.Canvas {
		t.Fatal("final canvases differ")
	}
}
func TestConnectErrors(t *testing.T) {
	for _, u := range []string{"http://example.com", "bad", "ws://user:pass@localhost/ws"} {
		if _, err := Connect(context.Background(), u); err == nil {
			t.Fatal("invalid URL accepted")
		}
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := Connect(ctx, "ws://"+addr+"/ws"); err == nil {
		t.Fatal("connection failure hidden")
	}
}
