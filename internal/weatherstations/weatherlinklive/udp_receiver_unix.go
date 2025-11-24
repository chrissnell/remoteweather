//go:build unix || darwin || linux || freebsd || openbsd || netbsd

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
	if err := syscall.SetsockoptInt(int(file.Fd()), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		conn.Close()
		return fmt.Errorf("failed to set SO_REUSEADDR: %w", err)
	}
	if err := syscall.SetsockoptInt(int(file.Fd()), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1); err != nil {
		conn.Close()
		return fmt.Errorf("failed to set SO_BROADCAST: %w", err)
	}

	s.udpConn = conn
	s.logger.Infof("UDP receiver started on port %d", s.broadcastPort)

	s.wg.Add(1)
	go s.receiveUDPPackets()

	return nil
}
