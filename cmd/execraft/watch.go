package main

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
)

func runWatch(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: execraft watch <base-url>")
	}
	baseURL := strings.TrimRight(args[0], "/")
	resp, err := http.Get(baseURL + "/events/stream")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("watch failed: %s", resp.Status)
	}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		fmt.Println(line)
	}
	return scanner.Err()
}
