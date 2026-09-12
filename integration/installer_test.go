package integration

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseInstaller(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the release installer supports Linux")
	}

	const version = "0.1.0"
	binaryContents := []byte("#!/bin/sh\nprintf 'smdctl test build\\n'\n")
	archiveName := fmt.Sprintf("smdctl_%s_linux_%s.tar.gz", version, runtime.GOARCH)

	for _, testCase := range []struct {
		name            string
		corruptChecksum bool
		wantError       string
	}{
		{name: "installs verified binary"},
		{name: "rejects checksum mismatch", corruptChecksum: true, wantError: "checksum mismatch"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			archiveDir := filepath.Join(root, "archive")
			if err := os.MkdirAll(archiveDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(archiveDir, "smdctl"), binaryContents, 0o755); err != nil {
				t.Fatal(err)
			}
			archivePath := filepath.Join(root, archiveName)
			if output, err := exec.Command("tar", "-czf", archivePath, "-C", archiveDir, "smdctl").CombinedOutput(); err != nil {
				t.Fatalf("create test archive: %v\n%s", err, output)
			}
			archive, err := os.ReadFile(archivePath)
			if err != nil {
				t.Fatal(err)
			}
			checksum := fmt.Sprintf("%x", sha256.Sum256(archive))
			if testCase.corruptChecksum {
				checksum = strings.Repeat("0", 64)
			}
			checksums := []byte(fmt.Sprintf("%s  %s\n", checksum, archiveName))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/" + archiveName:
					_, _ = w.Write(archive)
				case "/checksums.txt":
					_, _ = w.Write(checksums)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			home := filepath.Join(root, "home")
			if err := os.Mkdir(home, 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("HOME", home)
			t.Setenv("SMDCTL_VERSION", version)
			t.Setenv("SMDCTL_DOWNLOAD_URL", server.URL)
			t.Setenv("SMDCTL_INSTALL_DIR", "")
			cmd := exec.Command("sh", "../scripts/smdctl-installer.sh")
			output, err := cmd.CombinedOutput()
			if testCase.wantError != "" {
				if err == nil || !strings.Contains(string(output), testCase.wantError) {
					t.Fatalf("installer error = %v, output = %s; want %q", err, output, testCase.wantError)
				}
				if _, statErr := os.Stat(filepath.Join(home, ".local", "bin", "smdctl")); !os.IsNotExist(statErr) {
					t.Fatalf("installer left binary after checksum failure: %v", statErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("installer failed: %v\n%s", err, output)
			}

			installedPath := filepath.Join(home, ".local", "bin", "smdctl")
			installed, err := os.ReadFile(installedPath)
			if err != nil {
				t.Fatalf("read installed binary: %v", err)
			}
			if string(installed) != string(binaryContents) {
				t.Fatalf("installed binary contents = %q, want %q", installed, binaryContents)
			}
			if info, err := os.Stat(installedPath); err != nil || info.Mode().Perm()&0o111 == 0 {
				t.Fatalf("installed binary is not executable: info=%v err=%v", info, err)
			}
			homeEntries, err := os.ReadDir(home)
			if err != nil {
				t.Fatal(err)
			}
			if len(homeEntries) != 1 || homeEntries[0].Name() != ".local" {
				t.Fatalf("installer changed files outside .local/bin: %v", homeEntries)
			}
		})
	}
}
