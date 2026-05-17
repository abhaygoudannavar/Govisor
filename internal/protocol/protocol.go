package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
)

type Request struct {
	Command string `json:"command"`
	ID      string `json:"id,omitempty"`
	Cmd     string `json:"cmd,omitempty"`
	Lines   int    `json:"lines,omitempty"`
}

type ProcessInfo struct {
	ID      string `json:"id"`
	Cmd     string `json:"cmd"`
	PID     int    `json:"pid"`
	Running bool   `json:"running"`
}

type Response struct {
	OK        bool          `json:"ok"`
	Error     string        `json:"error,omitempty"`
	Message   string        `json:"message,omitempty"`
	Process   *ProcessInfo  `json:"process,omitempty"`
	Processes []ProcessInfo `json:"processes,omitempty"`
	Logs      []string      `json:"logs,omitempty"`
}

// Send writes a JSON object followed by a newline to the connection.
func Send(conn net.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %v", err)
	}
	data = append(data, '\n')
	_, err = conn.Write(data)
	return err
}

// Receive reads one newline-delimited JSON object from the connection.
func Receive(conn net.Conn, v any) error {
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return err
		}
		return fmt.Errorf("connection closed")
	}
	return json.Unmarshal(scanner.Bytes(), v)
}
