package weatherlinklive

import (
	"encoding/json"
	"net"
	"time"
)

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
		if err := s.udpConn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			s.logger.Errorf("Failed to set read deadline: %v", err)
			continue
		}
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
