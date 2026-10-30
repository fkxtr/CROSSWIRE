// Package server implements in-memory anonymous matchmaking and authoritative rooms.
package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/fkxtr/CROSSWIRE/internal/protocol"
	"github.com/gorilla/websocket"
)

type Config struct {
	Duration       time.Duration
	MaxConnections int
}
type Server struct {
	mu      sync.Mutex
	peers   map[*peer]bool
	rooms   map[*room]bool
	waiting *peer
	serial  uint64
	cfg     Config
	done    chan struct{}
	closed  bool
	wg      sync.WaitGroup
}
type peer struct {
	conn  *websocket.Conn
	out   chan protocol.Message
	done  chan struct{}
	once  sync.Once
	room  *room
	color uint8
}

func (p *peer) stop() { p.once.Do(func() { close(p.done); p.conn.Close() }) }
func (p *peer) send(m protocol.Message) {
	select {
	case <-p.done:
		return
	default:
	}
	select {
	case p.out <- m:
	default:
		p.stop()
	}
}

type event struct {
	p      *peer
	typeID string
	x, y   int
	left   bool
}
type room struct {
	server   *Server
	id       string
	peers    [2]*peer
	canvas   protocol.Canvas
	input    chan event
	done     chan struct{}
	deadline time.Time
}

func New(cfg Config) *Server {
	if cfg.Duration <= 0 {
		cfg.Duration = protocol.RoundSeconds * time.Second
	}
	if cfg.MaxConnections <= 0 {
		cfg.MaxConnections = 1024
	}
	return &Server{cfg: cfg, peers: make(map[*peer]bool), rooms: make(map[*room]bool), done: make(chan struct{})}
}

// Close terminates all owned sockets and waits for all room and handler goroutines.
func (s *Server) Close() {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		close(s.done)
		for p := range s.peers {
			p.stop()
		}
	}
	s.mu.Unlock()
	s.wg.Wait()
}
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/ws" {
		http.NotFound(w, req)
		return
	}
	s.mu.Lock()
	if s.closed || len(s.peers) >= s.cfg.MaxConnections {
		s.mu.Unlock()
		http.Error(w, "server unavailable", http.StatusServiceUnavailable)
		return
	}
	// Same-origin browser protection; native clients send no Origin.
	conn, err := (&websocket.Upgrader{HandshakeTimeout: 5 * time.Second}).Upgrade(w, req, nil)
	if err != nil {
		s.mu.Unlock()
		return
	}
	p := &peer{conn: conn, out: make(chan protocol.Message, 128), done: make(chan struct{})}
	s.peers[p] = true
	s.wg.Add(1)
	s.mu.Unlock()
	defer s.wg.Done()
	writerDone := make(chan struct{})
	go func() { defer close(writerDone); p.write() }()
	defer func() { s.remove(p); p.stop(); <-writerDone }()
	conn.SetReadLimit(512)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(45 * time.Second)) })
	joined := false
	window := time.Now()
	count := 0
	for {
		kind, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if time.Since(window) >= time.Second {
			window = time.Now()
			count = 0
		}
		count++
		if count > 100 {
			p.send(protocol.Message{Type: "error", Error: "rate limit exceeded"})
			return
		}
		var in protocol.Input
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if kind != websocket.TextMessage || dec.Decode(&in) != nil || dec.Decode(new(any)) != io.EOF {
			p.send(protocol.Message{Type: "error", Error: "invalid message"})
			continue
		}
		if !joined {
			if in.Type != "join" || in.X != nil || in.Y != nil {
				p.send(protocol.Message{Type: "error", Error: "join required"})
				continue
			}
			joined = true
			conn.SetReadDeadline(time.Now().Add(45 * time.Second))
			s.join(p)
			continue
		}
		validPoint := (in.Type == "draw" || in.Type == "erase") && in.X != nil && in.Y != nil && *in.X >= 0 && *in.X < protocol.Width && *in.Y >= 0 && *in.Y < protocol.Height
		validReset := in.Type == "reset" && in.X == nil && in.Y == nil
		if !validPoint && !validReset {
			p.send(protocol.Message{Type: "error", Error: "invalid action"})
			continue
		}
		s.mu.Lock()
		r := p.room
		s.mu.Unlock()
		if r == nil {
			p.send(protocol.Message{Type: "error", Error: "no active round"})
			continue
		}
		select {
		case r.input <- eventFromInput(p, in):
		case <-r.done:
			p.send(protocol.Message{Type: "error", Error: "round finished"})
		case <-s.done:
			return
		}
	}
}

func eventFromInput(p *peer, in protocol.Input) event {
	e := event{p: p, typeID: in.Type}
	if in.X != nil {
		e.x = *in.X
	}
	if in.Y != nil {
		e.y = *in.Y
	}
	return e
}
func (p *peer) write() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	defer p.stop()
	for {
		select {
		case m := <-p.out:
			p.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if p.conn.WriteJSON(m) != nil {
				return
			}
		case <-ticker.C:
			if p.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)) != nil {
				return
			}
		case <-p.done:
			return
		}
	}
}
func (s *Server) join(p *peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if s.waiting != nil {
		select {
		case <-s.waiting.done:
			s.waiting = nil
		default:
		}
	}
	if s.waiting == nil {
		s.waiting = p
		p.send(protocol.Message{Type: "waiting"})
		return
	}
	a := s.waiting
	s.waiting = nil
	s.serial++
	r := &room{server: s, id: fmt.Sprintf("%d", s.serial), peers: [2]*peer{a, p}, input: make(chan event, 256), done: make(chan struct{}), deadline: time.Now().Add(s.cfg.Duration)}
	s.rooms[r] = true
	for i, q := range r.peers {
		q.room = r
		q.color = uint8(i + 1)
		c := r.canvas
		q.send(protocol.Message{Type: "matched", Room: r.id, Player: q.color, Canvas: &c, Seconds: r.seconds()})
	}
	s.wg.Add(1)
	go func() { defer s.wg.Done(); r.run() }()
}
func (s *Server) remove(p *peer) {
	s.mu.Lock()
	delete(s.peers, p)
	if s.waiting == p {
		s.waiting = nil
	}
	r := p.room
	s.mu.Unlock()
	if r != nil {
		select {
		case r.input <- event{p: p, left: true}:
		case <-r.done:
		case <-s.done:
		}
	}
}
func (r *room) seconds() int {
	n := int(math.Ceil(time.Until(r.deadline).Seconds()))
	if n < 0 {
		return 0
	}
	return n
}
func (r *room) broadcast(m protocol.Message) {
	for _, p := range r.peers {
		p.send(m)
	}
}
func (r *room) snapshot(kind string) {
	c := r.canvas
	r.broadcast(protocol.Message{Type: kind, Room: r.id, Canvas: &c, Seconds: r.seconds()})
}
func (r *room) run() {
	defer func() {
		close(r.done)
		s := r.server
		s.mu.Lock()
		delete(s.rooms, r)
		for _, p := range r.peers {
			if p.room == r {
				p.room = nil
			}
		}
		s.mu.Unlock()
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	timer := time.NewTimer(time.Until(r.deadline))
	defer timer.Stop()
	left := make(map[*peer]bool)
	for {
		select {
		case <-r.server.done:
			return
		case <-timer.C:
			r.snapshot("finished")
			return
		case <-ticker.C:
			r.broadcast(protocol.Message{Type: "timer", Seconds: r.seconds()})
		case e := <-r.input:
			if e.left {
				left[e.p] = true
				for _, p := range r.peers {
					if p != e.p {
						p.send(protocol.Message{Type: "player_left"})
					}
				}
				if len(left) == 2 {
					return
				}
				continue
			}
			if !time.Now().Before(r.deadline) {
				r.snapshot("finished")
				return
			}
			if left[e.p] {
				continue
			}
			switch e.typeID {
			case "draw":
				r.canvas[e.y][e.x] = e.p.color
				r.snapshot("canvas_update")
			case "erase":
				r.canvas[e.y][e.x] = 0
				r.snapshot("canvas_update")
			case "reset":
				r.canvas = protocol.Canvas{}
				r.snapshot("canvas_reset")
			}
		}
	}
}
