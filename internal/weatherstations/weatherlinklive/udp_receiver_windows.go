//go:build windows

package weatherlinklive

import (
	"fmt"
	"net"
	"syscall"
)

// startUDPReceiver starts listening for UDP broadcast packets
func (s *Station) startUDPReceiver() error {
	addr := &net.UDPAddr{
		Port: s.broadcastPort,
		IP:   net.IPv4zero,
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return fmt.Errorf("failed to start UDP listener: %w", err)
	}

	// Set socket options for broadcast
	file, err := conn.File()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to get socket file: %w", err)
	}
	defer file.Close()

	// Enable address reuse and broadcast
	// On Windows, file.Fd() returns a syscall.Handle (uintptr)
	fd := syscall.Handle(file.Fd())
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		conn.Close()
		return fmt.Errorf("failed to set SO_REUSEADDR: %w", err)
	}
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1); err != nil {
		conn.Close()
		return fmt.Errorf("failed to set SO_BROADCAST: %w", err)
	}

	s.udpConn = conn
	s.logger.Infof("UDP receiver started on port %d", s.broadcastPort)

	s.wg.Add(1)
	go s.receiveUDPPackets()

	return nil
}
