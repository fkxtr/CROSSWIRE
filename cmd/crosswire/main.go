package main

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"flag"
	"fmt"
	"github.com/fkxtr/CROSSWIRE/internal/client"
	"os"
	"time"
)

func main() {
	address := os.Getenv("CROSSWIRE_SERVER")
	if address == "" {
		address = "ws://localhost:8080/ws"
	}
	server := flag.String("server", address, "WebSocket server URL")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	t, err := client.Connect(ctx, *server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "CROSSWIRE: %q\nStart crosswired or set --server / CROSSWIRE_SERVER.\n", err.Error())
		os.Exit(1)
	}
	err = run(t)
	if err != nil {
		fmt.Fprintf(os.Stderr, "CROSSWIRE: %q\n", err.Error())
		os.Exit(1)
	}
}
func run(t *client.Transport) error {
	defer t.Close()
	_, err := tea.NewProgram(client.New(t, os.Getenv("NO_COLOR") == "")).Run()
	return err
}
