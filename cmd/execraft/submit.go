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
	if len(args) < 2 {
		return fmt.Errorf("usage: execraft submit <base-url> <task-graph.json|->")
	}
	baseURL := strings.TrimRight(args[0], "/")
	target := args[1]

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

	resp, err := http.Post(baseURL+"/tasks", "application/json", bytes.NewReader(payload))
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
