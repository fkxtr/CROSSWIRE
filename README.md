# CROSSWIRE

**Two strangers. One terminal canvas. Sixty seconds.**

CROSSWIRE pairs two anonymous terminals for one minute of shared ASCII drawing.
The server owns the 40 × 16 canvas, assigns green and blue, orders drawing events,
and ends the round. No accounts, chat, database, analytics, or saved sessions.

Illustrative interface (not a captured session):

```text
  CROSSWIRE                         00:42

  +----------------------------------------+
  |                                        |
  |          +++++                         |
  |          +   +          ####           |
  |          +++++         #    #          |
  |                         ####           |
  |                                        |
  +----------------------------------------+

  YOU: GREEN (+)  STRANGER: BLUE (#)
  WASD / ARROWS: move  SPACE: draw  Q: quit
```

The actual canvas has 16 rows. Your local cursor is `@`. ASCII borders and distinct
`+` / `#` marks work without Unicode fonts and distinguish players without color.
Set `NO_COLOR=1` to disable application color styling. Use a terminal of at least
48 columns × 26 rows; smaller terminals display resize guidance.

## Run locally — three terminals

Install Go 1.26 or newer (CI and the prepared cloud environment use Go 1.27.3).
From the repository root, run:

Terminal 1:

```sh
go run ./cmd/crosswired
```

Terminal 2:

```sh
go run ./cmd/crosswire
```

Terminal 3:

```sh
go run ./cmd/crosswire
```

Both clients connect to `ws://localhost:8080/ws`. The first waits with
“Searching for another terminal...”. The second starts their shared 60-second
round. Draw in either terminal and observe the other terminal. The final canvas
stays visible until Q. Run the client again for another match.

| Key | Action |
| --- | --- |
| WASD / arrows | Move your local cursor |
| Space | Draw your mark in the current cell |
| Q / Ctrl+C | Leave and restore the terminal |

The last accepted draw at a cell wins. A disconnect notifies the other player,
who can continue until the deadline. Multiple independent pairs can play at once.

## Linux installation

Build locally using the commands below, then run:

```sh
./bin/crosswired-linux-amd64
./bin/crosswire-linux-amd64
```

To make the one-command names available, copy the Linux binaries as `crosswire`
and `crosswired` into a directory on your PATH. Tagged releases are configured to
produce these same artifacts; no public release or hosted server is assumed.

## Windows installation

Use Windows Terminal with PowerShell. Build on Windows with Go installed:

```powershell
go build -o crosswire.exe ./cmd/crosswire
go build -o crosswired.exe ./cmd/crosswired
.\crosswired.exe
# In each of two other terminal windows:
.\crosswire.exe
```

Native executables need no Go installation or external language runtime to run.
Windows executables are cross-compiled in Linux CI; native Windows terminal
behavior still needs verification on a Windows machine.

## Remote server

```sh
crosswire --server wss://example.com/ws
# Or:
export CROSSWIRE_SERVER=wss://example.com/ws
crosswire
```

PowerShell: `$env:CROSSWIRE_SERVER = 'wss://example.com/ws'`.
The flag overrides the environment variable. Connections have a five-second
setup timeout. A failed connection exits with a useful error; automatic
reconnection is intentionally absent. Restart the client to join a new round.

The server defaults to `:8080`; use `crosswired --addr 127.0.0.1:8080` when
running behind a reverse proxy on the same host. Serve public traffic over WSS.
For example, with Caddy installed separately and DNS pointed at your host:

```caddyfile
example.com {
    reverse_proxy 127.0.0.1:8080
}
```

Caddy handles TLS and WebSocket upgrades. Restrict direct access to port 8080.
Public deployment, domain setup, and certificates are not provisioned by this
repository. The service keeps no application request logs; configure proxy
logging and retention according to your privacy requirements.

## Build from source

From the repository root, on Linux/macOS:

```sh
mkdir -p bin
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o bin/crosswire-linux-amd64 ./cmd/crosswire
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o bin/crosswired-linux-amd64 ./cmd/crosswired
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -o bin/crosswire-windows-amd64.exe ./cmd/crosswire
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -o bin/crosswired-windows-amd64.exe ./cmd/crosswired
```

Dependencies are pinned in `go.mod` and `go.sum`. Bubble Tea v2 drives rendering,
keyboard handling and terminal restoration; Gorilla WebSocket carries JSON.

## Architecture and protocol

- `cmd/crosswire`: minimal client CLI.
- `cmd/crosswired`: HTTP/WebSocket server with signal shutdown.
- `internal/client`: bounded transport queues and Bubble Tea state machine.
- `internal/server`: protected matchmaking registry and one event loop per room.
- `internal/protocol`: shared fixed-size canvas and JSON message types.

A client sends `{"type":"join"}` once, then
`{"type":"draw","x":3,"y":4}`. Coordinates start at zero. It never sends
cursor movement, drawing text, identities, colors, or room identifiers.

The server sends `waiting`, `matched`, `canvas_update`, `timer`, `player_left`,
`finished`, and `error`. Both clients receive the same complete numeric canvas
snapshot after each accepted draw and at completion. The server assigns player
1 / green / `+` and player 2 / blue / `#`. Countdown messages report the ceiling
of remaining server time; local input cannot extend a round.

Rooms are removed at expiry, or once both players leave. Final canvases remain
only in connected clients. Connections waiting for Q still count toward the
server's default 1,024-connection limit.

## Testing

```sh
go test ./...
go test -race ./...
go vet ./...
```

Tests use dynamic local ports, real WebSockets, bounded timeouts and short
configurable server durations. They cover pairing, third-player isolation,
colors, simultaneous draws, last-writer wins, matching canvas snapshots, invalid
JSON/coordinates, message size and rate limits, disconnections, new connections,
expiry, cleanup, transport failures, and terminal state transitions. The runtime
default stays 60 seconds. Race tests require a supported C compiler and platform.

GitHub Actions runs tests, race checks, vet and all four builds. Pushing a `v*`
tag triggers the release workflow, which publishes binaries and SHA-256 checksums
only after validation succeeds. No release is created by a local build.

## Security and privacy

Only numeric coordinates are accepted. Unknown fields and message types are
rejected, and no server-provided text is interpolated into the terminal. Input
frames are limited to 512 bytes; clients are limited to 100 messages per second.
Outbound queues are bounded and slow consumers are disconnected. Ping/pong
heartbeats and socket deadlines detect dead connections. Browser upgrades use
Gorilla's same-origin default; native terminals send no Origin header.

Peers never receive IP addresses. The server necessarily sees connection
addresses at the network layer but does not forward or persist them. TLS
protects traffic in transit; this is not end-to-end encryption. A server operator
can observe the canvas. There is no permanent drawing storage or user profiling.
Per-connection limits are basic safeguards, not full DDoS protection; apply
connection limits and resource monitoring at the reverse proxy for public use.

## Inicio rápido (español)

Instala Go 1.26 o superior. Abre tres terminales en esta carpeta: ejecuta
`go run ./cmd/crosswired` en la primera y `go run ./cmd/crosswire` en las otras
dos. Muévete con WASD o las flechas y dibuja con espacio. La ronda dura 60
segundos; pulsa Q para salir. Para conectarte a un servidor remoto utiliza
`--server wss://tu-dominio/ws`.

## Contributing and license

See [CONTRIBUTING.md](CONTRIBUTING.md). Licensed under [MIT](LICENSE).
