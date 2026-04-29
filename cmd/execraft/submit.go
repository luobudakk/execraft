package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func runSubmit(args []string) error {
	token := strings.TrimSpace(os.Getenv("EXECRAFT_TOKEN"))
	positional := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--token":
			if i+1 >= len(args) {
				return fmt.Errorf("usage: execraft submit [--token <token>] <base-url> <task-graph.json|->")
			}
			token = strings.TrimSpace(args[i+1])
			i++
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) < 2 {
		return fmt.Errorf("usage: execraft submit [--token <token>] <base-url> <task-graph.json|->")
	}
	baseURL := strings.TrimRight(positional[0], "/")
	target := positional[1]

	var payload []byte
	var err error
	if target == "-" {
		payload, err = io.ReadAll(os.Stdin)
	} else {
		payload, err = os.ReadFile(target)
	}
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/tasks", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("x-execraft-token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	fmt.Println(string(out))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned %s", resp.Status)
	}
	return nil
}
