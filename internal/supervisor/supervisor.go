package supervisor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/abhaygoudannavar/govisor/internal/protocol"
)

const maxLogLines = 500

type ManagedProcess struct {
	ID      string
	CmdStr  string
	Cmd     *exec.Cmd
	Running bool
	Logs    *RingBuffer
}

// RingBuffer holds the last N lines of output.
type RingBuffer struct {
	lines []string
	mu    sync.Mutex
}

func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		lines: make([]string, 0, capacity),
	}
}

func (rb *RingBuffer) Add(line string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if len(rb.lines) >= maxLogLines {
		// drop the oldest line
		rb.lines = rb.lines[1:]
	}
	rb.lines = append(rb.lines, line)
}

func (rb *RingBuffer) Last(n int) []string {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if n <= 0 || len(rb.lines) == 0 {
		return nil
	}
	if n > len(rb.lines) {
		n = len(rb.lines)
	}
	start := len(rb.lines) - n
	out := make([]string, n)
	copy(out, rb.lines[start:])
	return out
}

type Supervisor struct {
	mu    sync.Mutex
	procs map[string]*ManagedProcess
}

func New() *Supervisor {
	return &Supervisor{
		procs: make(map[string]*ManagedProcess),
	}
}

func (s *Supervisor) Start(id, cmdStr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.procs[id]; exists {
		return fmt.Errorf("process %q already exists", id)
	}

	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	logs := NewRingBuffer(maxLogLines)

	// pipe stdout and stderr into the ring buffer
	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("pipe: %v", err)
	}
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return fmt.Errorf("start: %v", err)
	}

	proc := &ManagedProcess{
		ID:      id,
		CmdStr:  cmdStr,
		Cmd:     cmd,
		Running: true,
		Logs:    logs,
	}
	s.procs[id] = proc

	// read output in background
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := pr.Read(buf)
			if n > 0 {
				// split into lines, but keep it simple
				text := string(buf[:n])
				for _, line := range strings.Split(text, "\n") {
					if line != "" {
						logs.Add(line)
					}
				}
			}
			if err != nil {
				break
			}
		}
		pr.Close()
	}()

	// wait for exit in background
	go func() {
		cmd.Wait()
		pw.Close()
		s.mu.Lock()
		proc.Running = false
		s.mu.Unlock()
	}()

	return nil
}

func (s *Supervisor) Stop(id string) error {
	s.mu.Lock()
	proc, exists := s.procs[id]
	s.mu.Unlock()

	if !exists {
		return fmt.Errorf("process %q not found", id)
	}
	if !proc.Running {
		return fmt.Errorf("process %q already stopped", id)
	}

	// try SIGTERM first
	proc.Cmd.Process.Signal(syscall.SIGTERM)

	// TODO: add configurable timeout
	done := make(chan struct{})
	go func() {
		proc.Cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
		// exited cleanly
	case <-time.After(5 * time.Second):
		// force kill
		proc.Cmd.Process.Kill()
		<-done
	}

	s.mu.Lock()
	proc.Running = false
	s.mu.Unlock()
	return nil
}

func (s *Supervisor) Status(id string) (*protocol.ProcessInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	proc, exists := s.procs[id]
	if !exists {
		return nil, fmt.Errorf("process %q not found", id)
	}

	pid := 0
	if proc.Cmd.Process != nil {
		pid = proc.Cmd.Process.Pid
	}

	return &protocol.ProcessInfo{
		ID:      proc.ID,
		Cmd:     proc.CmdStr,
		PID:     pid,
		Running: proc.Running,
	}, nil
}

func (s *Supervisor) List() []protocol.ProcessInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	var list []protocol.ProcessInfo
	for _, proc := range s.procs {
		pid := 0
		if proc.Cmd.Process != nil {
			pid = proc.Cmd.Process.Pid
		}
		list = append(list, protocol.ProcessInfo{
			ID:      proc.ID,
			Cmd:     proc.CmdStr,
			PID:     pid,
			Running: proc.Running,
		})
	}
	return list
}

func (s *Supervisor) GetLogs(id string, lines int) ([]string, error) {
	s.mu.Lock()
	proc, exists := s.procs[id]
	s.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("process %q not found", id)
	}
	return proc.Logs.Last(lines), nil
}

// StopAll sends SIGTERM to every running process, then SIGKILL after timeout.
func (s *Supervisor) StopAll() {
	s.mu.Lock()
	var running []*ManagedProcess
	for _, proc := range s.procs {
		if proc.Running {
			running = append(running, proc)
		}
	}
	s.mu.Unlock()

	for _, proc := range running {
		proc.Cmd.Process.Signal(syscall.SIGTERM)
	}

	time.Sleep(5 * time.Second)

	for _, proc := range running {
		if proc.Running {
			proc.Cmd.Process.Kill()
		}
	}
}
