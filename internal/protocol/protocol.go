// Package protocol defines the only data exchanged between terminals and server.
package protocol

const Width = 40
const Height = 16
const RoundSeconds = 60

type Canvas [Height][Width]uint8

type Input struct {
	Type string `json:"type"`
	X    *int   `json:"x,omitempty"`
	Y    *int   `json:"y,omitempty"`
}

type Message struct {
	Type    string  `json:"type"`
	Room    string  `json:"room,omitempty"`
	Player  uint8   `json:"player,omitempty"`
	Canvas  *Canvas `json:"canvas,omitempty"`
	Seconds int     `json:"seconds"`
	Error   string  `json:"error,omitempty"`
}
