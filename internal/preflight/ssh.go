package preflight

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// DetectSSHPorts detects active SSH listening ports by inspecting sshd configs and listening sockets.
func DetectSSHPorts() ([]int, error) {
	portMap := make(map[int]bool)

	// 1. Check sshd configs
	configPorts := parseSSHConfigs()
	for _, p := range configPorts {
		portMap[p] = true
	}

	// 2. Check active socket listeners for sshd
	activePorts := parseActiveSSHSockets()
	for _, p := range activePorts {
		portMap[p] = true
	}

	// Fallback to 22 if none found
	if len(portMap) == 0 {
		portMap[22] = true
	}

	var ports []int
	for p := range portMap {
		if p > 0 && p <= 65535 {
			ports = append(ports, p)
		}
	}

	return ports, nil
}

// parseSSHConfigs parses /etc/ssh/sshd_config and /etc/ssh/sshd_config.d/*
func parseSSHConfigs() []int {
	var ports []int
	files := []string{"/etc/ssh/sshd_config"}

	matches, _ := filepath.Glob("/etc/ssh/sshd_config.d/*.conf")
	files = append(files, matches...)

	rePort := regexp.MustCompile(`(?i)^\s*Port\s+(\d+)`)

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "#") {
				continue
			}
			m := rePort.FindStringSubmatch(line)
			if len(m) == 2 {
				if p, err := strconv.Atoi(m[1]); err == nil {
					ports = append(ports, p)
				}
			}
		}
	}

	return ports
}

// parseActiveSSHSockets runs ss -tlpn or checks /proc to detect sshd listening port
func parseActiveSSHSockets() []int {
	var ports []int

	// Try `ss -tlpn`
	cmd := exec.Command("ss", "-tlpn")
	out, err := cmd.Output()
	if err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(out))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "sshd") || strings.Contains(line, ":ssh") {
				p := extractPortFromSS(line)
				if p > 0 {
					ports = append(ports, p)
				}
			}
		}
		if len(ports) > 0 {
			return ports
		}
	}

	// If ss didn't return, check /proc/net/tcp and /proc/net/tcp6
	ports = append(ports, parseProcNetTCP("/proc/net/tcp")...)
	ports = append(ports, parseProcNetTCP("/proc/net/tcp6")...)

	return ports
}

func extractPortFromSS(line string) int {
	fields := strings.Fields(line)
	if len(fields) >= 4 {
		// Local address is typically column 4 (0-indexed: 3 or 4)
		for _, col := range fields[3:] {
			if strings.Contains(col, ":") {
				parts := strings.Split(col, ":")
				portStr := parts[len(parts)-1]
				if p, err := strconv.Atoi(portStr); err == nil {
					return p
				}
			}
		}
	}
	return 0
}

// parseProcNetTCP reads Linux /proc/net/tcp listening ports
func parseProcNetTCP(path string) []int {
	var ports []int
	data, err := os.ReadFile(path)
	if err != nil {
		return ports
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		// Format: sl local_address rem_address st ...
		// st == 0A means TCP_LISTEN
		if len(fields) >= 4 && fields[3] == "0A" {
			localAddr := fields[1]
			parts := strings.Split(localAddr, ":")
			if len(parts) == 2 {
				portHex := parts[1]
				if p, err := strconv.ParseInt(portHex, 16, 64); err == nil {
					// Check if this port is commonly 22 or > 1024
					if p == 22 || p > 1024 {
						ports = append(ports, int(p))
					}
				}
			}
		}
	}

	return ports
}

// FormatSSHPorts returns a user-friendly string for detected SSH ports
func FormatSSHPorts(ports []int) string {
	if len(ports) == 0 {
		return "None"
	}
	var str []string
	for _, p := range ports {
		str = append(str, fmt.Sprintf("TCP/%d", p))
	}
	return strings.Join(str, ", ")
}
