package bench

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// agentPrompt builds a deterministic prompt of roughly agentTokens tokens: a
// listing of small functions followed by a question about one of them. Real
// coding agents send requests this size on every turn, so how long the model
// takes to answer one (including loading cold) is the number that matters.
func agentPrompt(functions int) (prompt string, want int) {
	var b strings.Builder
	b.WriteString("Below is part of a code base. Read it, then answer the question at the end.\n\n")
	target := functions * 6 / 10
	for i := 0; i < functions; i++ {
		a, c := 3+(i*7919)%89, 11+(i*104729)%97
		fmt.Fprintf(&b, "// file: pkg/calc/module_%04d.go\n", i)
		fmt.Fprintf(&b, "// Compute%04d applies the linear rule used by pipeline stage %d.\n", i, i%13)
		fmt.Fprintf(&b, "func Compute%04d(x int) int {\n\treturn x*%d + %d\n}\n\n", i, a, c)
		if i == target {
			want = 2*a + c
		}
	}
	fmt.Fprintf(&b, "Question: what does Compute%04d(2) return? Reply with just the number.\n", target)
	return b.String(), want
}

// chatResult is what the agent-sized request measured.
type chatResult struct {
	Seconds      float64
	PromptTokens int
	GenTokens    int
	Reply        string
	Correct      bool
	Err          error
}

// chatResponse is the part of a llama-server chat completion pill reads.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Timings struct {
		PredictedPerSecond float64 `json:"predicted_per_second"`
	} `json:"timings"`
}

// postChat sends one non-streaming chat completion and returns the decoded
// response and how long it took.
func (r *Runner) postChat(ctx context.Context, payload map[string]any, timeout time.Duration) (chatResponse, float64, error) {
	var out chatResponse
	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.Router.BaseURL()+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return out, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return out, time.Since(start).Seconds(), err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	secs := time.Since(start).Seconds()
	if resp.StatusCode != http.StatusOK {
		return out, secs, fmt.Errorf("chat completion: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw[:min(len(raw), 300)])))
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
		return out, secs, fmt.Errorf("chat completion: unreadable response")
	}
	return out, secs, nil
}

// agentRequest sends the agent-sized prompt through the router and times it.
func (r *Runner) agentRequest(ctx context.Context, model string, timeout time.Duration) chatResult {
	prompt, want := agentPrompt(r.PromptFunctions)
	out, secs, err := r.postChat(ctx, map[string]any{
		"model":                model,
		"messages":             []map[string]string{{"role": "user", "content": prompt}},
		"max_tokens":           64,
		"stream":               false,
		"chat_template_kwargs": map[string]any{"enable_thinking": false},
	}, timeout)
	res := chatResult{Seconds: secs, Err: err}
	if err != nil {
		return res
	}
	res.Reply = strings.TrimSpace(out.Choices[0].Message.Content)
	res.PromptTokens, res.GenTokens = out.Usage.PromptTokens, out.Usage.CompletionTokens
	res.Correct = strings.Contains(res.Reply, fmt.Sprint(want))
	if res.Reply == "" {
		res.Err = fmt.Errorf("the model returned an empty reply")
	}
	return res
}

// decodeTokens is how many tokens the speed probe generates: enough that the
// rate reflects steady decoding rather than the first token's overhead.
const decodeTokens = 128

// decodeSpeed measures generation speed on the already-loaded model. The
// agent-sized request's own rate is useless for this: its reply is a few
// tokens, so the figure is mostly start-up overhead. ignore_eos (a
// llama-server extension) forces exactly decodeTokens tokens.
func (r *Runner) decodeSpeed(ctx context.Context, model string, timeout time.Duration) (tokPerSec float64, tokens int, err error) {
	out, _, err := r.postChat(ctx, map[string]any{
		"model":                model,
		"messages":             []map[string]string{{"role": "user", "content": "Count upwards from 1, one number per line."}},
		"max_tokens":           decodeTokens,
		"ignore_eos":           true,
		"stream":               false,
		"chat_template_kwargs": map[string]any{"enable_thinking": false},
	}, timeout)
	if err != nil {
		return 0, 0, err
	}
	return out.Timings.PredictedPerSecond, out.Usage.CompletionTokens, nil
}

// randomToken makes a code the model cannot guess, so a correct answer proves
// it actually used a tool to read the file.
func randomToken() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "PILL-" + strings.ToUpper(hex.EncodeToString(b))
}
