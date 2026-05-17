# govisor

A simple process supervisor that talks over a Unix socket. I built this because I wanted to understand how process managers like supervisord work under the hood — specifically the IPC part where a daemon manages child processes and a CLI client sends it commands.

## How it works

There are two binaries:

- **govisord** — the daemon. It listens on `/tmp/govisor.sock` and manages child processes.
- **govisor** — the CLI client. It connects to the socket, sends a JSON command, and prints the response.

Communication uses newline-delimited JSON over a Unix domain socket. Nothing fancy.

```
 ┌──────────┐    JSON over Unix socket    ┌──────────┐
 │  govisor │ ──────────────────────────> │ govisord │
 │  (CLI)   │ <────────────────────────── │ (daemon) │
 └──────────┘                             └────┬─────┘
                                               │
                                          manages child
                                          processes via
                                          os/exec
```

## Build

```bash
make build
```

Binaries go into `bin/`.

## Usage

Start the daemon first:

```bash
./bin/govisord
```

Then in another terminal:

```bash
# start a process
./bin/govisor start --id web --cmd "python3 -m http.server 8080"

# check on it
./bin/govisor status --id web

# see what's running
./bin/govisor list

# grab some logs
./bin/govisor logs --id web --lines 20

# stop it
./bin/govisor stop --id web
```

## Demo

Build and run the tests:

```
$ make build
go build -o bin/govisord ./cmd/govisord
go build -o bin/govisor ./cmd/govisor

$ make test
=== RUN   TestStartAndStatus
--- PASS: TestStartAndStatus (0.00s)
=== RUN   TestStartDuplicate
--- PASS: TestStartDuplicate (0.00s)
=== RUN   TestStopProcess
--- PASS: TestStopProcess (0.10s)
=== RUN   TestStatusNotFound
--- PASS: TestStatusNotFound (0.00s)
=== RUN   TestList
--- PASS: TestList (0.00s)
=== RUN   TestRingBuffer
--- PASS: TestRingBuffer (0.00s)
=== RUN   TestGetLogs
--- PASS: TestGetLogs (0.50s)
PASS
ok      github.com/abhaygoudannavar/govisor/internal/supervisor 0.617s
```

Start the daemon in one terminal, then use the CLI in another:

```
$ ./bin/govisord
2026/05/17 18:20:39 govisord listening on /tmp/govisor.sock
```

```
$ ./bin/govisor start --id pingtest --cmd "ping -c 10 localhost"
started pingtest

$ ./bin/govisor status --id pingtest
ID:          pingtest
Command:     ping -c 10 localhost
PID:         3873
State:       running

$ ./bin/govisor list
ID              PID      STATE      COMMAND
------------------------------------------------------------
pingtest        3873     running    ping -c 10 localhost

$ ./bin/govisor logs --id pingtest --lines 5
PING localhost (127.0.0.1) 56(84) bytes of data.
64 bytes from localhost (127.0.0.1): icmp_seq=1 ttl=64 time=0.410 ms
64 bytes from localhost (127.0.0.1): icmp_seq=2 ttl=64 time=0.206 ms

$ ./bin/govisor start --id sleeper --cmd "sleep 300"
started sleeper

$ ./bin/govisor list
ID              PID      STATE      COMMAND
------------------------------------------------------------
pingtest        3873     running    ping -c 10 localhost
sleeper         3903     running    sleep 300

$ ./bin/govisor stop --id sleeper
stopped sleeper

$ ./bin/govisor list
ID              PID      STATE      COMMAND
------------------------------------------------------------
pingtest        3873     running    ping -c 10 localhost
sleeper         3903     stopped    sleep 300
```

## Running tests

```bash
make test
```

Tests cover the supervisor internals — spawning, stopping, duplicate detection, the log ring buffer, etc.

## How the internals work

- The daemon keeps a `map[string]*ManagedProcess` where each entry tracks the exec.Cmd, running state, and a ring buffer of the last 500 lines of stdout/stderr.
- When you start a process, it pipes stdout and stderr through an os.Pipe and a goroutine reads from it into the ring buffer.
- Stopping sends SIGTERM first, waits 5 seconds, then SIGKILL if still alive.
- On SIGINT/SIGTERM to the daemon itself, it tries to gracefully stop all managed processes before exiting.

## Limitations

Things I haven't done yet:

- No process restart policies (auto-restart on crash, backoff, etc.)
- The socket path is hardcoded to `/tmp/govisor.sock` — should be configurable
- No authentication on the socket. Anyone who can reach the file can send commands.
- Logs are in-memory only. If the daemon restarts, all log history is gone.
- No support for setting environment variables or working directory for child processes.
- The ring buffer capacity is hardcoded at 500 lines.
- Process entries stick around after the process dies. There's no "remove" or auto-cleanup.

## Dependencies

None. Standard library only.
