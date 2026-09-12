package systemd

import (
	"regexp"
	"strconv"
	"strings"
)

var portPattern = regexp.MustCompile(`(?:--?port[=\s]|:)(\d+)`)
var portEnvironmentKeys = []string{"PORT", "BIND_PORT", "HTTP_PORT", "HTTPS_PORT", "SERVER_PORT", "LISTEN_PORT", "APP_PORT"}

func NeedsElevatedPort(svc *Service) bool {
	for _, key := range portEnvironmentKeys {
		if n, e := strconv.Atoi(svc.Environment[key]); e == nil && n > 0 && n < 1024 {
			return true
		}
	}
	full := svc.Command + " " + strings.Join(svc.Args, " ")
	for _, m := range portPattern.FindAllStringSubmatch(full, -1) {
		if n, e := strconv.Atoi(m[1]); e == nil && n > 0 && n < 1024 {
			return true
		}
	}
	return false
}
func DetectMode(svc *Service, forceSystem bool) Mode {
	if forceSystem || NeedsElevatedPort(svc) {
		return System
	}
	return User
}
