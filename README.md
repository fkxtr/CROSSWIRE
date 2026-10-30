# CROSSWIRE

**Two strangers. One terminal canvas. Sixty seconds.**

CROSSWIRE pairs two anonymous terminals for one minute of shared ASCII drawing.
The server owns the 40 × 16 canvas, assigns green and blue, orders drawing events,
and ends the round. No accounts, chat, database, analytics, or saved sessions.

Illustrative interface (not a captured session):

```text
  CROSSWIRE  [LIVE    ]             00:42

  +----------------------------------------+
  |                                        |
  |          +++++                         |
  |          +   +          ####           |
  |          +++++         #    #          |
  |                         ####           |
  |                                        |
  +----------------------------------------+

  YOU  GREEN (+)       PARTNER  BLUE (#)
  MOVE  WASD/arrows   DRAW  Space   ERASE  E
  RESET R             QUIT  Q
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
| E / Backspace / Delete | Erase the current cell |
| R | Reset the complete shared canvas |
| Q / Ctrl+C | Leave and restore the terminal |

The last accepted action at a cell wins. Erase and reset actions are validated by
the server and synchronized to both terminals. Reset clears the canvas but does
not restart the 60-second timer. A disconnect notifies the other player, who can
continue until the deadline. Multiple independent pairs can play at once.

## Instalación y uso en Linux (español)

### 1. Requisitos

Necesitas Git y Go 1.26 o una versión posterior. En Debian, Ubuntu o Kali puedes
instalar Git y las herramientas básicas con:

```sh
sudo apt update
sudo apt install -y git ca-certificates curl
```

Comprueba la versión de Go:

```sh
go version
```

Si el comando no existe o la versión es anterior a 1.26, instala una versión
actual desde [go.dev/dl](https://go.dev/dl/) siguiendo las instrucciones oficiales
para Linux. Cierra y abre el terminal después de actualizar `PATH`, y vuelve a
ejecutar `go version` antes de continuar.

### 2. Descargar y compilar CROSSWIRE

Los siguientes comandos crean `~/CROSSWIRE`; no los ejecutes desde una copia
distinta del repositorio:

```sh
cd ~
git clone https://github.com/fkxtr/CROSSWIRE.git
cd CROSSWIRE
go mod download
go test ./...

mkdir -p bin
CGO_ENABLED=0 go build -trimpath -o bin/crosswired ./cmd/crosswired
CGO_ENABLED=0 go build -trimpath -o bin/crosswire ./cmd/crosswire
```

Comprueba que existen los dos ejecutables:

```sh
ls -lh bin/crosswire bin/crosswired
```

Si ya habías clonado el proyecto, actualízalo desde su carpeta y recompila:

```sh
cd ~/CROSSWIRE
git pull --ff-only
go mod download
CGO_ENABLED=0 go build -trimpath -o bin/crosswired ./cmd/crosswired
CGO_ENABLED=0 go build -trimpath -o bin/crosswire ./cmd/crosswire
```

### 3. Probar dos jugadores en el mismo ordenador

Abre tres terminales. Debe existir exactamente un servidor.

Terminal 1 — servidor:

```sh
cd ~/CROSSWIRE
./bin/crosswired --addr 127.0.0.1:8080
```

Debe aparecer `CROSSWIRE server listening on 127.0.0.1:8080`. Mantén esa
terminal abierta.

Terminal 2 — primer jugador:

```sh
cd ~/CROSSWIRE
./bin/crosswire --server ws://127.0.0.1:8080/ws
```

Terminal 3 — segundo jugador:

```sh
cd ~/CROSSWIRE
./bin/crosswire --server ws://127.0.0.1:8080/ws
```

La ruta `/ws` al final de la dirección es obligatoria. El primer cliente mostrará
`Searching for another terminal...`; la partida empieza cuando se conecta el
segundo. No ejecutes un servidor para cada jugador.

Durante la partida usa WASD o las flechas para moverte, espacio para dibujar, `E`
para borrar una celda, `R` para limpiar el lienzo compartido y `Q` para salir. El
reinicio del lienzo no reinicia el contador.

### 4. Conectar desde otro ordenador de la red local

En el ordenador que ejecuta el servidor, consulta su IP:

```sh
hostname -I
```

Inicia el servidor aceptando conexiones de red:

```sh
./bin/crosswired --addr 0.0.0.0:8080
```

En cada cliente sustituye `IP_DEL_SERVIDOR` por esa IP:

```sh
./bin/crosswire --server ws://IP_DEL_SERVIDOR:8080/ws
```

No expongas directamente este puerto a Internet. Para un servidor público usa
WSS detrás de un proxy TLS, como se explica en [Remote server](#remote-server).

### 5. Solución de problemas

- Si ambos clientes siguen buscando, confirma que usan exactamente la misma URL,
  incluido `/ws`, y que solo hay un servidor con `ss -ltnp | grep 8080`.
- Si aparece `connection refused`, inicia primero `crosswired` y revisa que el
  puerto coincida con la URL de los clientes.
- Si aparece `address already in use`, ya existe otro proceso en el puerto 8080.
  Localízalo con `ss -ltnp | grep 8080` o elige otro puerto para servidor y clientes.
- Si `go build` no encuentra `go.mod`, entra primero en la carpeta del repositorio
  con `cd ~/CROSSWIRE`.
- Si la interfaz pide más espacio, amplía la terminal hasta al menos 48 × 26.

## Linux installation

The detailed Spanish section above includes cloning, compilation, local-network
usage and troubleshooting. For the shortest same-computer test after building,
run one server and two clients in three terminals:

```sh
./bin/crosswired --addr 127.0.0.1:8080
./bin/crosswire --server ws://127.0.0.1:8080/ws
```

Run the client command in two separate terminals. Tagged releases are configured
to produce platform-specific artifacts, but no hosted public server is assumed.

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

A client sends `{"type":"join"}` once, followed by coordinate actions such as
`{"type":"draw","x":3,"y":4}` or `{"type":"erase","x":3,"y":4}`.
`{"type":"reset"}` clears the shared canvas. Coordinates start at zero. The
client never sends cursor movement, drawing text, identities, colors, or room
identifiers.

The server sends `waiting`, `matched`, `canvas_update`, `canvas_reset`, `timer`,
`player_left`, `finished`, and `error`. Both clients receive the same complete numeric canvas
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
colors, simultaneous draws, erase/reset synchronization, last-writer wins,
matching canvas snapshots, invalid JSON/coordinates, message size and rate limits, disconnections, new connections,
expiry, cleanup, transport failures, and terminal state transitions. The runtime
default stays 60 seconds. Race tests require a supported C compiler and platform.

GitHub Actions runs tests, race checks, vet and all four builds. Pushing a `v*`
tag triggers the release workflow, which publishes binaries and SHA-256 checksums
only after validation succeeds. No release is created by a local build.

## Security and privacy

Only known actions and valid numeric coordinates are accepted. Reset accepts no
coordinates. Unknown fields and message types are rejected, and no server-provided text is interpolated into the terminal. Input
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

Consulta [Instalación y uso en Linux (español)](#instalación-y-uso-en-linux-español)
para descargar, compilar y solucionar problemas. Muévete con WASD o las flechas,
dibuja con espacio, borra con `E`, limpia el lienzo con `R` y sal con `Q`.

## Contributing and license

See [CONTRIBUTING.md](CONTRIBUTING.md). Licensed under [MIT](LICENSE).
