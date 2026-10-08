package cmd

import (
	"context"
	"net/http"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

// execFake starts the fake llama-server on the env's port with a foreign preset file.
func execFake(t *testing.T, e *env, preset string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(e.fakes.LlamaServer, "--port", strconv.Itoa(e.port), "--models-preset", preset)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if portAnswers(e.port) {
			return cmd
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("fake llama-server did not start")
	return nil
}

func portAnswers(port int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, _ := newRequest(ctx, "http://127.0.0.1:"+strconv.Itoa(port)+"/health")
	resp, err := httpClient.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == 200
}

var httpClient = &http.Client{Timeout: time.Second}

func newRequest(ctx context.Context, url string) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
}
