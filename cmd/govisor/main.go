package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/abhaygoudannavar/govisor/internal/protocol"
)

const socketPath = "/tmp/govisor.sock"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcmd := os.Args[1]

	switch subcmd {
	case "start":
		doStart(os.Args[2:])
	case "stop":
		doStop(os.Args[2:])
	case "status":
		doStatus(os.Args[2:])
	case "list":
		doList()
	case "logs":
		doLogs(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", subcmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: govisor <command> [flags]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "commands:")
	fmt.Fprintln(os.Stderr, "  start   --id NAME --cmd COMMAND")
	fmt.Fprintln(os.Stderr, "  stop    --id NAME")
	fmt.Fprintln(os.Stderr, "  status  --id NAME")
	fmt.Fprintln(os.Stderr, "  list")
	fmt.Fprintln(os.Stderr, "  logs    --id NAME [--lines N]")
}

func sendRequest(req *protocol.Request) *protocol.Response {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not connect to govisord: %v\n", err)
		fmt.Fprintln(os.Stderr, "is the daemon running?")
		os.Exit(1)
	}
	defer conn.Close()

	if err := protocol.Send(conn, req); err != nil {
		fmt.Fprintf(os.Stderr, "send: %v\n", err)
		os.Exit(1)
	}

	var resp protocol.Response
	if err := protocol.Receive(conn, &resp); err != nil {
		fmt.Fprintf(os.Stderr, "receive: %v\n", err)
		os.Exit(1)
	}
	return &resp
}

func doStart(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	id := fs.String("id", "", "process ID")
	cmd := fs.String("cmd", "", "command to run")
	fs.Parse(args)

	if *id == "" || *cmd == "" {
		fmt.Fprintln(os.Stderr, "start requires --id and --cmd")
		os.Exit(1)
	}

	resp := sendRequest(&protocol.Request{
		Command: "start",
		ID:      *id,
		Cmd:     *cmd,
	})

	if !resp.OK {
		fmt.Fprintf(os.Stderr, "error: %s\n", resp.Error)
		os.Exit(1)
	}
	fmt.Println(resp.Message)
}

func doStop(args []string) {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)
	id := fs.String("id", "", "process ID")
	fs.Parse(args)

	if *id == "" {
		fmt.Fprintln(os.Stderr, "stop requires --id")
		os.Exit(1)
	}

	resp := sendRequest(&protocol.Request{
		Command: "stop",
		ID:      *id,
	})

	if !resp.OK {
		fmt.Fprintf(os.Stderr, "error: %s\n", resp.Error)
		os.Exit(1)
	}
	fmt.Println(resp.Message)
}

func doStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	id := fs.String("id", "", "process ID")
	fs.Parse(args)

	if *id == "" {
		fmt.Fprintln(os.Stderr, "status requires --id")
		os.Exit(1)
	}

	resp := sendRequest(&protocol.Request{
		Command: "status",
		ID:      *id,
	})

	if !resp.OK {
		fmt.Fprintf(os.Stderr, "error: %s\n", resp.Error)
		os.Exit(1)
	}

	p := resp.Process
	state := "stopped"
	if p.Running {
		state = "running"
	}
	fmt.Printf("%-12s %s\n", "ID:", p.ID)
	fmt.Printf("%-12s %s\n", "Command:", p.Cmd)
	fmt.Printf("%-12s %d\n", "PID:", p.PID)
	fmt.Printf("%-12s %s\n", "State:", state)
}

func doList() {
	resp := sendRequest(&protocol.Request{Command: "list"})

	if !resp.OK {
		fmt.Fprintf(os.Stderr, "error: %s\n", resp.Error)
		os.Exit(1)
	}

	if len(resp.Processes) == 0 {
		fmt.Println("no managed processes")
		return
	}

	// simple table
	fmt.Printf("%-15s %-8s %-10s %s\n", "ID", "PID", "STATE", "COMMAND")
	fmt.Println(strings.Repeat("-", 60))
	for _, p := range resp.Processes {
		state := "stopped"
		if p.Running {
			state = "running"
		}
		fmt.Printf("%-15s %-8d %-10s %s\n", p.ID, p.PID, state, p.Cmd)
	}
}

func doLogs(args []string) {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	id := fs.String("id", "", "process ID")
	lines := fs.Int("lines", 50, "number of log lines")
	fs.Parse(args)

	if *id == "" {
		fmt.Fprintln(os.Stderr, "logs requires --id")
		os.Exit(1)
	}

	resp := sendRequest(&protocol.Request{
		Command: "logs",
		ID:      *id,
		Lines:   *lines,
	})

	if !resp.OK {
		fmt.Fprintf(os.Stderr, "error: %s\n", resp.Error)
		os.Exit(1)
	}

	if len(resp.Logs) == 0 {
		fmt.Println("(no log output)")
		return
	}

	for _, line := range resp.Logs {
		fmt.Println(line)
	}

	// putting this here just in case i want to add json output later
	_ = json.Marshal
}
