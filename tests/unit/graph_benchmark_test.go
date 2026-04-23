package unit

import (
	"fmt"
	"testing"

	"github.com/jinziqi/execraft/internal/domain"
)

func BenchmarkValidateGraph(b *testing.B) {
	tasks := make([]domain.TaskSpec, 0, 200)
	for i := 0; i < 200; i++ {
		id := fmt.Sprintf("t-%d", i)
		spec := domain.TaskSpec{ID: id, Kind: "echo"}
		if i > 0 {
			spec.DependsOn = []string{fmt.Sprintf("t-%d", i-1)}
		}
		tasks = append(tasks, spec)
	}
	graph := domain.TaskGraph{Tasks: tasks}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := domain.ValidateGraph(graph); err != nil {
			b.Fatal(err)
		}
	}
}
