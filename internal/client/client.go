// Package client contains the transport and Bubble Tea state machine.
package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/fkxtr/CROSSWIRE/internal/protocol"
	"github.com/gorilla/websocket"
)

type failure struct{}
type Transport struct {
	conn     *websocket.Conn
	incoming chan tea.Msg
	outgoing chan protocol.Input
	done     chan struct{}
	once     sync.Once
	wg       sync.WaitGroup
}

func Connect(ctx context.Context, address string) (*Transport, error) {
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") || u.Host == "" || u.User != nil {
		return nil, errors.New("server must be a ws:// or wss:// URL without credentials")
	}
	dial := websocket.Dialer{HandshakeTimeout: 5 * time.Second, Proxy: http.ProxyFromEnvironment}
	conn, resp, err := dial.DialContext(ctx, address, nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("cannot connect to server: %w", err)
	}
	conn.SetReadLimit(16384)
	conn.SetReadDeadline(time.Now().Add(45 * time.Second))
	conn.SetPingHandler(func(data string) error {
		conn.SetReadDeadline(time.Now().Add(45 * time.Second))
		return conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(5*time.Second))
	})
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteJSON(protocol.Input{Type: "join"}); err != nil {
		conn.Close()
		return nil, err
	}
	t := &Transport{conn: conn, incoming: make(chan tea.Msg, 128), outgoing: make(chan protocol.Input, 64), done: make(chan struct{})}
	t.wg.Add(2)
	go t.read()
	go t.write()
	return t, nil
}
func (t *Transport) stop()  { t.once.Do(func() { close(t.done); t.conn.Close() }) }
func (t *Transport) Close() { t.stop(); t.wg.Wait() }
func (t *Transport) read() {
	defer t.wg.Done()
	for {
		var m protocol.Message
		if t.conn.ReadJSON(&m) != nil {
			select {
			case t.incoming <- failure{}:
			case <-t.done:
			}
			return
		}
		select {
		case t.incoming <- m:
		case <-t.done:
			return
		}
	}
}
func (t *Transport) write() {
	defer t.wg.Done()
	for {
		select {
		case in := <-t.outgoing:
			t.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if t.conn.WriteJSON(in) != nil {
				select {
				case t.incoming <- failure{}:
				case <-t.done:
				}
				return
			}
		case <-t.done:
			return
		}
	}
}
func (t *Transport) sendAction(action string, x, y *int) bool {
	select {
	case <-t.done:
		return false
	default:
	}
	select {
	case t.outgoing <- protocol.Input{Type: action, X: x, Y: y}:
		return true
	default:
		return false
	}
}
func (t *Transport) draw(x, y int) bool  { return t.sendAction("draw", &x, &y) }
func (t *Transport) erase(x, y int) bool { return t.sendAction("erase", &x, &y) }
func (t *Transport) reset() bool         { return t.sendAction("reset", nil, nil) }
func (t *Transport) receive() tea.Msg {
	select {
	case m := <-t.incoming:
		return m
	case <-t.done:
		return failure{}
	}
}

type Model struct {
	transport                    *Transport
	canvas                       protocol.Canvas
	player                       uint8
	x, y, width, height, seconds int
	state, status                string
	color                        bool
}

func New(t *Transport, color bool) Model {
	return Model{transport: t, state: "waiting", status: "Searching for another terminal...", seconds: 60, color: color}
}
func (m Model) Init() tea.Cmd { return m.transport.receive }
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = v.Width
		m.height = v.Height
	case failure:
		if m.state != "finished" {
			m.state = "disconnected"
			m.status = "Connection lost. Restart crosswire to reconnect."
		}
		return m, nil
	case protocol.Message:
		switch v.Type {
		case "waiting":
			m.state = "waiting"
		case "matched":
			if (v.Player != 1 && v.Player != 2) || v.Canvas == nil {
				m.state = "disconnected"
				m.status = "Invalid server response."
				return m, nil
			}
			m.state = "playing"
			m.player = v.Player
			m.status = "Make something together."
		case "canvas_update":
		case "canvas_reset":
			m.status = "Canvas cleared. Start a new drawing."
		case "timer":
		case "player_left":
			m.status = "Stranger left. You can finish the drawing."
		case "finished":
			m.state = "finished"
			m.status = "Time is up. Your shared canvas. Q to exit."
		case "error":
			m.status = "Server rejected the last action."
		default:
			return m, m.transport.receive
		}
		if v.Canvas != nil {
			m.canvas = *v.Canvas
		}
		if v.Type == "timer" || v.Type == "matched" || v.Type == "finished" || v.Type == "canvas_update" || v.Type == "canvas_reset" {
			m.seconds = max(0, min(60, v.Seconds))
		}
		return m, m.transport.receive
	case tea.KeyPressMsg:
		key := v.String()
		if key == "q" || key == "ctrl+c" {
			return m, tea.Quit
		}
		if m.state != "playing" || m.width < 48 || m.height < 26 {
			return m, nil
		}
		switch key {
		case "w", "up":
			m.y = max(0, m.y-1)
		case "s", "down":
			m.y = min(protocol.Height-1, m.y+1)
		case "a", "left":
			m.x = max(0, m.x-1)
		case "d", "right":
			m.x = min(protocol.Width-1, m.x+1)
		case "space", " ":
			if !m.transport.draw(m.x, m.y) {
				m.status = "Network busy. Try drawing again."
			}
		case "e", "backspace", "delete":
			if !m.transport.erase(m.x, m.y) {
				m.status = "Network busy. Try erasing again."
			}
		case "r":
			if !m.transport.reset() {
				m.status = "Network busy. Try resetting again."
			}
		}
	}
	return m, nil
}
func (m Model) ink(s string, p uint8) string {
	if !m.color {
		return s
	}
	switch p {
	case 1:
		return "\x1b[92m" + s + "\x1b[0m"
	case 2:
		return "\x1b[94m" + s + "\x1b[0m"
	}
	return s
}
func (m Model) View() tea.View {
	var b strings.Builder
	if m.width < 48 || m.height < 26 {
		b.WriteString("CROSSWIRE\nTerminal too small: need 48 x 26.\nResize the terminal. Q: quit.")
	} else {
		phase := "WAITING"
		if m.state == "playing" {
			phase = "LIVE"
		} else if m.state == "finished" {
			phase = "FINISHED"
		} else if m.state == "disconnected" {
			phase = "OFFLINE"
		}
		fmt.Fprintf(&b, "  CROSSWIRE  [%-8s]             %02d:%02d\n\n", phase, m.seconds/60, m.seconds%60)
		if m.state == "waiting" || m.state == "disconnected" {
			fmt.Fprintf(&b, "  %s\n\n  Q: quit\n", m.status)
		} else {
			b.WriteString("  +" + strings.Repeat("-", protocol.Width) + "+\n")
			for y, row := range m.canvas {
				b.WriteString("  |")
				for x, p := range row {
					s := " "
					if p == 1 {
						s = "+"
					} else if p == 2 {
						s = "#"
					}
					if m.state == "playing" && x == m.x && y == m.y {
						s = "@"
						p = m.player
					}
					b.WriteString(m.ink(s, p))
				}
				b.WriteString("|\n")
			}
			b.WriteString("  +" + strings.Repeat("-", protocol.Width) + "+\n\n")
			own, other := "GREEN (+)", "BLUE (#)"
			if m.player == 2 {
				own, other = other, own
			}
			fmt.Fprintf(&b, "  YOU  %s PARTNER  %s\n", m.ink(fmt.Sprintf("%-16s", own), m.player), m.ink(other, 3-m.player))
			b.WriteString("  MOVE  WASD/arrows   DRAW  Space   ERASE  E\n")
			b.WriteString("  RESET R             QUIT  Q\n")
			b.WriteString("  " + m.status)
		}
	}
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}
