package management

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// SerialPortInfo represents information about a serial port
type SerialPortInfo struct {
	Device       string `json:"device"`
	Name         string `json:"name,omitempty"`
	Description  string `json:"description,omitempty"`
	VendorID     string `json:"vendor_id,omitempty"`
	ProductID    string `json:"product_id,omitempty"`
	SerialNum    string `json:"serial_number,omitempty"`
	Type         string `json:"type"`
	Manufacturer string `json:"manufacturer,omitempty"`
}

// SystemInfo represents basic system information
type SystemInfo struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Hostname     string `json:"hostname"`
	Timestamp    int64  `json:"timestamp"`
}

// GetSerialPorts returns available serial ports on the system
func (h *Handlers) GetSerialPorts(w http.ResponseWriter, r *http.Request) {
	ports, err := h.enumerateSerialPorts()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to enumerate serial ports", err)
		return
	}

	response := map[string]interface{}{
		"ports":     ports,
		"count":     len(ports),
		"timestamp": time.Now().Unix(),
		"os":        runtime.GOOS,
	}

	h.sendJSON(w, response)
}

// GetSystemInfo returns basic system information
func (h *Handlers) GetSystemInfo(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()

	info := SystemInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		Hostname:     hostname,
		Timestamp:    time.Now().Unix(),
	}

	h.sendJSON(w, info)
}

// enumerateSerialPorts discovers available serial ports based on the operating system
func (h *Handlers) enumerateSerialPorts() ([]SerialPortInfo, error) {
	var ports []SerialPortInfo
	var err error

	switch runtime.GOOS {
	case "linux":
		ports, err = h.enumerateLinuxSerialPorts()
	case "darwin":
		ports, err = h.enumerateDarwinSerialPorts()
	case "windows":
		ports, err = h.enumerateWindowsSerialPorts()
	case "freebsd", "netbsd", "openbsd":
		ports, err = h.enumerateBSDSerialPorts()
	default:
		return []SerialPortInfo{}, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if err != nil {
		return ports, err
	}

	// Sort ports by device name for consistent output
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Device < ports[j].Device
	})

	return ports, nil
}

// enumerateLinuxSerialPorts finds serial ports on Linux systems
func (h *Handlers) enumerateLinuxSerialPorts() ([]SerialPortInfo, error) {
	var ports []SerialPortInfo

	// Common Linux serial device patterns
	patterns := []string{
		"/dev/ttyUSB*",        // USB-to-serial adapters
		"/dev/ttyACM*",        // USB CDC ACM devices
		"/dev/ttyS*",          // Traditional serial ports
		"/dev/ttyAMA*",        // ARM UART (Raspberry Pi)
		"/dev/ttyO*",          // OMAP UART (BeagleBone)
		"/dev/ttymxc*",        // i.MX UART
		"/dev/ttyTHS*",        // Tegra High Speed UART
		"/dev/serial/by-id/*", // Persistent device names
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, device := range matches {
			if info, err := os.Stat(device); err == nil && !info.IsDir() {
				// Resolve symlinks for /dev/serial/by-id/*
				realDevice := device
				if strings.Contains(device, "/dev/serial/by-id/") {
					if resolved, err := filepath.EvalSymlinks(device); err == nil {
						realDevice = resolved
					}
				}

				port := SerialPortInfo{
					Device: realDevice,
					Name:   filepath.Base(realDevice),
					Type:   h.getLinuxPortType(realDevice),
				}

				// Try to get additional info from sysfs
				h.enrichLinuxPortInfo(&port)

				// Avoid duplicates
				if !h.containsPort(ports, port.Device) {
					ports = append(ports, port)
				}
			}
		}
	}

	return ports, nil
}

// enumerateDarwinSerialPorts finds serial ports on macOS systems
func (h *Handlers) enumerateDarwinSerialPorts() ([]SerialPortInfo, error) {
	var ports []SerialPortInfo
	var cuPorts []SerialPortInfo
	var ttyPorts []SerialPortInfo

	// macOS device patterns
	patterns := []string{
		"/dev/cu.*",  // Callout devices (preferred for communication)
		"/dev/tty.*", // Terminal devices
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, device := range matches {
			// Skip system/internal devices
			if h.isDarwinSystemDevice(device) {
				continue
			}

			if info, err := os.Stat(device); err == nil && !info.IsDir() {
				port := SerialPortInfo{
					Device:      device,
					Name:        filepath.Base(device),
					Type:        h.getDarwinPortType(device),
					Description: h.getDarwinPortDescription(device),
				}

				// Try to get USB device info using ioreg
				h.enrichDarwinPortInfo(&port)

				// Separate cu.* and tty.* devices
				if strings.HasPrefix(filepath.Base(device), "cu.") {
					cuPorts = append(cuPorts, port)
				} else if strings.HasPrefix(filepath.Base(device), "tty.") {
					ttyPorts = append(ttyPorts, port)
				}
			}
		}
	}

	// Add cu.* devices first (preferred)
	ports = append(ports, cuPorts...)

	// Add tty.* devices only if there's no corresponding cu.* device
	for _, ttyPort := range ttyPorts {
		ttyBaseName := strings.TrimPrefix(filepath.Base(ttyPort.Device), "tty.")
		hasCorrespondingCu := false

		for _, cuPort := range cuPorts {
			cuBaseName := strings.TrimPrefix(filepath.Base(cuPort.Device), "cu.")
			if cuBaseName == ttyBaseName {
				hasCorrespondingCu = true
				break
			}
		}

		if !hasCorrespondingCu {
			ports = append(ports, ttyPort)
		}
	}

	return ports, nil
}

// enumerateWindowsSerialPorts finds serial ports on Windows systems
func (h *Handlers) enumerateWindowsSerialPorts() ([]SerialPortInfo, error) {
	var ports []SerialPortInfo

	// Try to use wmic to get detailed COM port information
	if wmicPorts, err := h.getWindowsWMICPorts(); err == nil && len(wmicPorts) > 0 {
		return wmicPorts, nil
	}

	// Fallback: try PowerShell Get-WmiObject method
	if psPorts, err := h.getWindowsPowerShellPorts(); err == nil && len(psPorts) > 0 {
		return psPorts, nil
	}

	// Final fallback: check common COM ports by trying to open them
	for i := 1; i <= 20; i++ { // Reduced from 256 to 20 for performance
		device := fmt.Sprintf("COM%d", i)
		if h.isWindowsPortAvailable(device) {
			port := SerialPortInfo{
				Device: device,
				Name:   device,
				Type:   "COM",
			}
			ports = append(ports, port)
		}
	}

	return ports, nil
}

// enumerateBSDSerialPorts finds serial ports on BSD systems
func (h *Handlers) enumerateBSDSerialPorts() ([]SerialPortInfo, error) {
	var ports []SerialPortInfo

	// BSD device patterns
	patterns := []string{
		"/dev/cuau*", // FreeBSD USB serial
		"/dev/cuad*", // FreeBSD serial
		"/dev/ttyU*", // OpenBSD/NetBSD USB serial
		"/dev/tty0*", // Traditional serial
		"/dev/cua*",  // Callout devices
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, device := range matches {
			if info, err := os.Stat(device); err == nil && !info.IsDir() {
				port := SerialPortInfo{
					Device: device,
					Name:   filepath.Base(device),
					Type:   h.getBSDPortType(device),
				}
				ports = append(ports, port)
			}
		}
	}

	return ports, nil
}

// Helper functions for port type detection
func (h *Handlers) getLinuxPortType(device string) string {
	if strings.Contains(device, "ttyUSB") {
		return "USB-Serial"
	} else if strings.Contains(device, "ttyACM") {
		return "USB-CDC"
	} else if strings.Contains(device, "ttyS") {
		return "RS-232"
	} else if strings.Contains(device, "ttyAMA") {
		return "ARM-UART"
	} else if strings.Contains(device, "ttyO") {
		return "OMAP-UART"
	} else if strings.Contains(device, "ttymxc") {
		return "i.MX-UART"
	} else if strings.Contains(device, "ttyTHS") {
		return "Tegra-UART"
	}
	return "Serial"
}

func (h *Handlers) getDarwinPortType(device string) string {
	if strings.Contains(device, "usbserial") || strings.Contains(device, "usbmodem") {
		return "USB-Serial"
	} else if strings.Contains(device, "Bluetooth") {
		return "Bluetooth"
	} else if strings.Contains(device, "serial") {
		return "Serial"
	}
	return "Unknown"
}

func (h *Handlers) getBSDPortType(device string) string {
	if strings.Contains(device, "cuau") || strings.Contains(device, "ttyU") {
		return "USB-Serial"
	} else if strings.Contains(device, "cuad") || strings.Contains(device, "tty0") {
		return "Serial"
	}
	return "Unknown"
}

// Helper functions for device descriptions
func (h *Handlers) getDarwinPortDescription(device string) string {
	name := filepath.Base(device)
	if strings.Contains(name, "usbserial") {
		return "USB Serial Device"
	} else if strings.Contains(name, "usbmodem") {
		return "USB Modem Device"
	} else if strings.Contains(name, "Bluetooth") {
		return "Bluetooth Serial Port"
	} else if strings.Contains(name, "serial") {
		return "Serial Port"
	}
	return "Serial Device"
}

// Helper functions for system-specific enrichment
func (h *Handlers) enrichLinuxPortInfo(port *SerialPortInfo) {
	deviceName := filepath.Base(port.Device)

	// Try to get USB vendor/product info from sysfs
	if strings.Contains(deviceName, "ttyUSB") || strings.Contains(deviceName, "ttyACM") {
		// Look for USB device info in sysfs
		sysfsPath := fmt.Sprintf("/sys/class/tty/%s/device", deviceName)
		if h.readUSBInfoFromSysfs(sysfsPath, port) {
			return
		}
	}
}

func (h *Handlers) enrichDarwinPortInfo(port *SerialPortInfo) {
	// Use ioreg to get USB device information
	if strings.Contains(port.Device, "usbserial") || strings.Contains(port.Device, "usbmodem") {
		h.getDarwinUSBInfo(port)
	}
}

// Utility functions
func (h *Handlers) containsPort(ports []SerialPortInfo, device string) bool {
	for _, port := range ports {
		if port.Device == device {
			return true
		}
	}
	return false
}

func (h *Handlers) isDarwinSystemDevice(device string) bool {
	systemDevices := []string{
		"Bluetooth",
		"console",
		"ptmx",
		"stdin",
		"stdout",
		"stderr",
	}

	for _, sys := range systemDevices {
		if strings.Contains(device, sys) {
			return true
		}
	}
	return false
}

func (h *Handlers) readUSBInfoFromSysfs(sysfsPath string, port *SerialPortInfo) bool {
	// This is a simplified implementation
	// In practice, you'd traverse the USB device hierarchy
	vendorPath := filepath.Join(sysfsPath, "../idVendor")
	productPath := filepath.Join(sysfsPath, "../idProduct")
	manufacturerPath := filepath.Join(sysfsPath, "../manufacturer")

	if vendor, err := os.ReadFile(vendorPath); err == nil {
		port.VendorID = strings.TrimSpace(string(vendor))
	}

	if product, err := os.ReadFile(productPath); err == nil {
		port.ProductID = strings.TrimSpace(string(product))
	}

	if manufacturer, err := os.ReadFile(manufacturerPath); err == nil {
		port.Manufacturer = strings.TrimSpace(string(manufacturer))
	}

	return port.VendorID != "" || port.ProductID != ""
}

func (h *Handlers) getDarwinUSBInfo(port *SerialPortInfo) {
	// Use ioreg to get USB device information
	// This is a simplified version - full implementation would parse ioreg output
	cmd := exec.Command("ioreg", "-p", "IOUSB", "-l")
	if output, err := cmd.Output(); err == nil {
		// Parse ioreg output to extract USB device info
		// This is complex and would require proper XML/plist parsing
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "USB Vendor Name") {
				// Extract vendor name (simplified)
				if parts := strings.Split(line, "="); len(parts) > 1 {
					port.Manufacturer = strings.Trim(strings.TrimSpace(parts[1]), `"`)
				}
				break
			}
		}
	}
}

func (h *Handlers) getWindowsWMICPorts() ([]SerialPortInfo, error) {
	// Use wmic to get COM port information on Windows
	cmd := exec.Command("wmic", "path", "Win32_SerialPort", "get", "DeviceID,Name,Description,PNPDeviceID", "/format:csv")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var ports []SerialPortInfo
	lines := strings.Split(string(output), "\n")

	for _, line := range lines[1:] { // Skip header
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, ",")
		if len(fields) >= 4 && strings.TrimSpace(fields[1]) != "" {
			deviceID := strings.TrimSpace(fields[1])
			description := strings.TrimSpace(fields[2])
			// name := strings.TrimSpace(fields[3]) // Not used currently
			pnpID := ""
			if len(fields) >= 5 {
				pnpID = strings.TrimSpace(fields[4])
			}

			port := SerialPortInfo{
				Device:      deviceID,
				Name:        deviceID,
				Description: description,
				Type:        "COM",
			}

			// Extract vendor/product info from PNP Device ID if available
			if pnpID != "" {
				h.parseWindowsPNPID(pnpID, &port)
			}

			ports = append(ports, port)
		}
	}

	return ports, nil
}

func (h *Handlers) getWindowsPowerShellPorts() ([]SerialPortInfo, error) {
	// Use PowerShell Get-WmiObject to get COM port information
	cmd := exec.Command("powershell", "-Command",
		"Get-WmiObject -Class Win32_SerialPort | Select-Object DeviceID, Name, Description, PNPDeviceID | ConvertTo-Csv -NoTypeInformation")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var ports []SerialPortInfo
	lines := strings.Split(string(output), "\n")

	for i, line := range lines {
		if i == 0 { // Skip header
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse CSV line (simple parsing, may need improvement for complex descriptions)
		fields := strings.Split(line, ",")
		if len(fields) >= 3 {
			deviceID := strings.Trim(strings.TrimSpace(fields[0]), `"`)
			// name := strings.Trim(strings.TrimSpace(fields[1]), `"`) // Not used currently
			description := strings.Trim(strings.TrimSpace(fields[2]), `"`)
			pnpID := ""
			if len(fields) >= 4 {
				pnpID = strings.Trim(strings.TrimSpace(fields[3]), `"`)
			}

			if deviceID != "" {
				port := SerialPortInfo{
					Device:      deviceID,
					Name:        deviceID,
					Description: description,
					Type:        "COM",
				}

				// Extract vendor/product info from PNP Device ID if available
				if pnpID != "" {
					h.parseWindowsPNPID(pnpID, &port)
				}

				ports = append(ports, port)
			}
		}
	}

	return ports, nil
}

func (h *Handlers) parseWindowsPNPID(pnpID string, port *SerialPortInfo) {
	// Parse Windows PNP Device ID to extract vendor/product information
	// Example: USB\VID_10C4&PID_EA60\0001
	if strings.Contains(pnpID, "VID_") && strings.Contains(pnpID, "PID_") {
		// Extract VID and PID
		parts := strings.Split(pnpID, "\\")
		for _, part := range parts {
			if strings.Contains(part, "VID_") && strings.Contains(part, "PID_") {
				subParts := strings.Split(part, "&")
				for _, subPart := range subParts {
					if strings.HasPrefix(subPart, "VID_") {
						port.VendorID = strings.TrimPrefix(subPart, "VID_")
					} else if strings.HasPrefix(subPart, "PID_") {
						port.ProductID = strings.TrimPrefix(subPart, "PID_")
					}
				}
			}
		}

		// Set type based on the bus type
		if strings.HasPrefix(pnpID, "USB\\") {
			port.Type = "USB-Serial"
		} else if strings.HasPrefix(pnpID, "FTDIBUS\\") {
			port.Type = "FTDI-USB"
		}
	}
}

func (h *Handlers) isWindowsPortAvailable(device string) bool {
	// Try to use PowerShell to check if COM port exists
	cmd := exec.Command("powershell", "-Command",
		fmt.Sprintf("[System.IO.Ports.SerialPort]::GetPortNames() -contains '%s'", device))

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	result := strings.TrimSpace(string(output))
	return strings.ToLower(result) == "true"
}
