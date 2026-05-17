package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/abhaygoudannavar/govisor/internal/protocol"
	"github.com/abhaygoudannavar/govisor/internal/supervisor"
)

const socketPath = "/tmp/govisor.sock"

func main() {
	// clean up old socket file if it exists
	os.Remove(socketPath)

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	defer os.Remove(socketPath)

	sup := supervisor.New()

	// handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutting down, stopping all processes...")
		sup.StopAll()
		ln.Close()
		os.Remove(socketPath)
		os.Exit(0)
	}()

	log.Printf("govisord listening on %s", socketPath)

	for {
		conn, err := ln.Accept()
		if err != nil {
			// listener was closed during shutdown
			break
		}
		go handleConn(conn, sup)
	}
}

func handleConn(conn net.Conn, sup *supervisor.Supervisor) {
	defer conn.Close()

	var req protocol.Request
	if err := protocol.Receive(conn, &req); err != nil {
		log.Printf("bad request: %v", err)
		return
	}

	var resp protocol.Response

	switch req.Command {
	case "start":
		if req.ID == "" || req.Cmd == "" {
			resp = protocol.Response{OK: false, Error: "id and cmd are required"}
		} else if err := sup.Start(req.ID, req.Cmd); err != nil {
			resp = protocol.Response{OK: false, Error: err.Error()}
		} else {
			resp = protocol.Response{OK: true, Message: "started " + req.ID}
		}

	case "stop":
		if err := sup.Stop(req.ID); err != nil {
			resp = protocol.Response{OK: false, Error: err.Error()}
		} else {
			resp = protocol.Response{OK: true, Message: "stopped " + req.ID}
		}

	case "status":
		info, err := sup.Status(req.ID)
		if err != nil {
			resp = protocol.Response{OK: false, Error: err.Error()}
		} else {
			resp = protocol.Response{OK: true, Process: info}
		}

	case "list":
		list := sup.List()
		resp = protocol.Response{OK: true, Processes: list}

	case "logs":
		lines := req.Lines
		if lines <= 0 {
			lines = 50
		}
		logs, err := sup.GetLogs(req.ID, lines)
		if err != nil {
			resp = protocol.Response{OK: false, Error: err.Error()}
		} else {
			resp = protocol.Response{OK: true, Logs: logs}
		}

	default:
		resp = protocol.Response{OK: false, Error: "unknown command: " + req.Command}
	}

	protocol.Send(conn, &resp)
}
