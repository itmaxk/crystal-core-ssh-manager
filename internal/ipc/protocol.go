package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
)

type MessageType string

const (
	MsgSessionStarted MessageType = "session_started"
	MsgSessionEnded   MessageType = "session_ended"
	MsgFocusOmnibar   MessageType = "focus_omnibar"
)

type Message struct {
	Type      MessageType `json:"type"`
	SessionID string      `json:"session_id,omitempty"`
	HostID    string      `json:"host_id,omitempty"`
	PID       int         `json:"pid,omitempty"`
	ExitCode  int         `json:"exit_code,omitempty"`
}

type Handler func(msg Message)

type Server struct {
	listener net.Listener
	handler  Handler
}

func NewServer(handler Handler) (*Server, error) {
	ln, err := listenTransport()
	if err != nil {
		return nil, fmt.Errorf("ipc listen: %w", err)
	}
	return &Server{listener: ln, handler: handler}, nil
}

func (s *Server) Start(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		_ = s.listener.Close()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var msg Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err == nil && s.handler != nil {
			s.handler(msg)
		}
	}
}

func Send(msg Message) error {
	conn, err := dialTransport()
	if err != nil {
		return fmt.Errorf("ipc dial: %w", err)
	}
	defer conn.Close()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = conn.Write(append(data, '\n'))
	return err
}
