package domain

import "fmt"

type ExecutionPlan struct {
	RunID      string
	Tasks      map[string]TaskSpec
	InDegree   map[string]int
	Dependents map[string][]string
}

func BuildPlan(runID string, graph TaskGraph) (*ExecutionPlan, error) {
	if runID == "" {
		return nil, fmt.Errorf("run id is required")
	}
	if err := ValidateGraph(graph); err != nil {
		return nil, err
	}
	plan := &ExecutionPlan{
		RunID:      runID,
		Tasks:      make(map[string]TaskSpec, len(graph.Tasks)),
		InDegree:   make(map[string]int, len(graph.Tasks)),
		Dependents: make(map[string][]string, len(graph.Tasks)),
	}
	for _, t := range graph.Tasks {
		plan.Tasks[t.ID] = t
		for _, dep := range t.DependsOn {
			plan.InDegree[t.ID]++
			plan.Dependents[dep] = append(plan.Dependents[dep], t.ID)
		}
	}
	return plan, nil
}
