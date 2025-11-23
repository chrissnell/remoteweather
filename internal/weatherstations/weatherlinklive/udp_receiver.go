package weatherlinklive

import (
	"encoding/json"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/chrissnell/remoteweather/internal/types"
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

// receiveUDPPackets continuously receives and processes UDP packets
func (s *Station) receiveUDPPackets() {
	defer s.wg.Done()
	defer s.logger.Info("UDP receiver stopped")

	buffer := make([]byte, 4096)

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Set read deadline to allow periodic context checking
		s.udpConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, addr, err := s.udpConn.ReadFrom(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Timeout is expected, continue
				continue
			}
			s.logger.Errorf("UDP read error: %v", err)
			continue
		}

		// Parse the UDP address to extract the IP (without port)
		udpAddr, ok := addr.(*net.UDPAddr)
		if !ok {
			continue
		}
		sourceIP := udpAddr.IP.String()

		// Filter by source IP (must match configured hostname)
		// Note: hostname should be configured as IP address for WLL devices
		if sourceIP != s.config.Hostname {
			s.logger.Debugf("Ignoring packet from %s (expecting %s)", sourceIP, s.config.Hostname)
			continue
		}

		// Parse JSON packet
		var data CurrentConditionsData
		if err := json.Unmarshal(buffer[:n], &data); err != nil {
			s.logger.Errorf("JSON parse error: %v", err)
			continue
		}

		// Transform and distribute reading
		reading := s.transformToReading(&data)
		if reading != nil {
			s.ReadingDistributor <- *reading
			s.setConnected(true)
		}
	}
}

// stopUDPReceiver stops the UDP receiver
func (s *Station) stopUDPReceiver() {
	if s.udpConn != nil {
		s.udpConn.Close()
		s.udpConn = nil
	}
}

// restartUDPReceiver restarts the UDP receiver on a new port
func (s *Station) restartUDPReceiver(newPort int) error {
	s.stopUDPReceiver()
	s.broadcastPort = newPort
	return s.startUDPReceiver()
}
