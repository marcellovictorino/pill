package pi

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// Binary locates the pi executable. PILL_PI overrides the lookup (tests point
// it at a fake).
func Binary() (string, error) {
	if p := os.Getenv("PILL_PI"); p != "" {
		return p, nil
	}
	p, err := exec.LookPath("pi")
	if err != nil {
		return "", fmt.Errorf("pi not found on PATH")
	}
	return p, nil
}

// Args builds Pi's argument list: pill's defaults first (--model, --thinking
// off), skipping any the caller already passed so their choice wins.
func Args(model string, user []string) []string {
	hasModel, hasThinking := false, false
	for _, a := range user {
		if a == "--" {
			break
		}
		name, _, _ := strings.Cut(a, "=")
		switch name {
		case "--model":
			hasModel = true
		case "--thinking":
			hasThinking = true
		}
	}
	var args []string
	if !hasModel {
		args = append(args, "--model", ProviderKey+"/"+model)
	}
	if !hasThinking {
		args = append(args, "--thinking", "off")
	}
	return append(args, user...)
}

// ExecFunc replaces the current process; syscall.Exec in production, a
// recorder in tests (a real exec would end the test binary).
var ExecFunc = syscall.Exec

// Exec replaces this process with pi. Unlike running pi as a child, exec means
// pill disappears: Pi owns the terminal, receives signals directly and its
// exit code is the shell's. It only returns on failure.
func Exec(bin string, args []string) error {
	return ExecFunc(bin, append([]string{"pi"}, args...), os.Environ())
}

// Version runs `pi --version`.
func Version(bin string) (string, error) {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
