package platform

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Info contains details of the host platform.
type Info struct {
	OS          string `json:"os"`
	Distro      string `json:"distro"`
	Version     string `json:"version"`
	VersionID   string `json:"version_id"`
	Arch        string `json:"arch"`
	IsRoot      bool   `json:"is_root"`
	HasSystemd  bool   `json:"has_systemd"`
	HasUFW      bool   `json:"has_ufw"`
	FreeDiskMB  uint64 `json:"free_disk_mb"`
	TotalMemMB  uint64 `json:"total_mem_mb"`
}

// Detect gathers platform information.
func Detect() (*Info, error) {
	info := &Info{
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		IsRoot:     os.Geteuid() == 0,
		HasSystemd: checkSystemd(),
		HasUFW:     checkBinary("ufw"),
	}

	distro, ver, verID := parseOSRelease()
	info.Distro = distro
	info.Version = ver
	info.VersionID = verID

	diskMB, _ := getDiskFreeMB("/")
	info.FreeDiskMB = diskMB

	memMB, _ := getMemTotalMB()
	info.TotalMemMB = memMB

	return info, nil
}

func checkSystemd() bool {
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		return true
	}
	_, err := exec.LookPath("systemctl")
	return err == nil
}

func checkBinary(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

func parseOSRelease() (distro, version, versionID string) {
	paths := []string{"/etc/os-release", "/usr/lib/os-release"}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "ID=") {
				distro = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
			} else if strings.HasPrefix(line, "VERSION=") {
				version = strings.Trim(strings.TrimPrefix(line, "VERSION="), "\"")
			} else if strings.HasPrefix(line, "VERSION_ID=") {
				versionID = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
			}
		}
		if distro != "" {
			break
		}
	}
	if distro == "" {
		distro = runtime.GOOS
	}
	return distro, version, versionID
}

func getMemTotalMB() (uint64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					return kb / 1024, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("MemTotal not found")
}

// getDiskFreeMB calls df or syscall to get free disk space in MB
func getDiskFreeMB(path string) (uint64, error) {
	cmd := exec.Command("df", "-m", "--output=avail", path)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) >= 2 {
		val := strings.TrimSpace(lines[len(lines)-1])
		mb, err := strconv.ParseUint(val, 10, 64)
		if err == nil {
			return mb, nil
		}
	}
	return 0, fmt.Errorf("could not parse disk free output")
}
