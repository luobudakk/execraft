package unit

import (
	"testing"

	"github.com/jinziqi/execraft/internal/domain"
)

func TestValidateGraph(t *testing.T) {
	t.Run("valid dag", func(t *testing.T) {
		err := domain.ValidateGraph(domain.TaskGraph{
			Tasks: []domain.TaskSpec{
				{ID: "a", Kind: "echo"},
				{ID: "b", Kind: "echo", DependsOn: []string{"a"}},
			},
		})
		if err != nil {
			t.Fatalf("expected valid graph, got %v", err)
		}
	})

	t.Run("cycle", func(t *testing.T) {
		err := domain.ValidateGraph(domain.TaskGraph{
			Tasks: []domain.TaskSpec{
				{ID: "a", Kind: "echo", DependsOn: []string{"b"}},
				{ID: "b", Kind: "echo", DependsOn: []string{"a"}},
			},
		})
		if err == nil {
			t.Fatal("expected cycle error")
		}
	})
}
