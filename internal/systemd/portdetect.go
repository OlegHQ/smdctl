package systemd

import (
	"regexp"
	"strconv"
	"strings"
)

// PortDetector handles detection of privileged ports
type PortDetector struct {
	envVarNames []string
	portPattern *regexp.Regexp
}

// NewPortDetector creates a new port detector
func NewPortDetector() *PortDetector {
	return &PortDetector{
		envVarNames: []string{
			"PORT", "BIND_PORT", "HTTP_PORT", "HTTPS_PORT",
			"SERVER_PORT", "LISTEN_PORT", "APP_PORT",
		},
		portPattern: regexp.MustCompile(`(?:--?port[=\s]|:)(\d+)`),
	}
}

// NeedsElevatedPort checks if service requires privileged port (< 1024)
func (pd *PortDetector) NeedsElevatedPort(svc *Service) bool {
	// Check environment variables
	for _, envKey := range pd.envVarNames {
		if portStr, ok := svc.Environment[envKey]; ok {
			if port, err := strconv.Atoi(portStr); err == nil {
				if port < 1024 && port > 0 {
					return true
				}
			}
		}
	}

	// Check command and args for port references
	fullCmd := svc.Command + " " + strings.Join(svc.Args, " ")
	matches := pd.portPattern.FindAllStringSubmatch(fullCmd, -1)
	for _, match := range matches {
		if len(match) > 1 {
			if port, err := strconv.Atoi(match[1]); err == nil {
				if port < 1024 && port > 0 {
					return true
				}
			}
		}
	}

	return false
}

// DetectMode determines the appropriate systemd mode for a service
func DetectMode(svc *Service, forceSystem bool) SystemdMode {
	if forceSystem {
		return ModeSystem
	}

	detector := NewPortDetector()
	if detector.NeedsElevatedPort(svc) {
		return ModeSystem
	}

	return ModeUser
}
