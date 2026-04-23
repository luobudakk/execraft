package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func (r *Router) handleEventStream(w http.ResponseWriter, req *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	offset, _ := strconv.ParseInt(req.URL.Query().Get("offset"), 10, 64)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-req.Context().Done():
			return
		case <-ticker.C:
			events, latest, err := r.events.ListSince(offset)
			if err != nil {
				return
			}
			for _, ev := range events {
				data, _ := json.Marshal(ev)
				_, _ = fmt.Fprintf(w, "event: %s\n", ev.Type)
				_, _ = fmt.Fprintf(w, "id: %d\n", ev.Offset)
				_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			}
			if latest > offset {
				offset = latest
			}
			flusher.Flush()
		}
	}
}
