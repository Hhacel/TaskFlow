package handlers

import "fmt"

// validateTaskGraph ensures every dependency reference exists among the
// submitted tasks and that the resulting dependency graph is acyclic (DAG).
func validateTaskGraph(tasks []TaskDefinition) error {
	byRef := make(map[string]TaskDefinition, len(tasks))
	for _, t := range tasks {
		if _, exists := byRef[t.Ref]; exists {
			return fmt.Errorf("duplicate task ref %q", t.Ref)
		}
		byRef[t.Ref] = t
	}

	for _, t := range tasks {
		for _, dep := range t.DependsOn {
			if _, ok := byRef[dep]; !ok {
				return fmt.Errorf("task %q depends on unknown ref %q", t.Ref, dep)
			}
			if dep == t.Ref {
				return fmt.Errorf("task %q cannot depend on itself", t.Ref)
			}
		}
	}

	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)
	state := make(map[string]int, len(tasks))

	var visit func(ref string) error
	visit = func(ref string) error {
		switch state[ref] {
		case visited:
			return nil
		case visiting:
			return fmt.Errorf("dependency cycle detected involving task %q", ref)
		}
		state[ref] = visiting
		for _, dep := range byRef[ref].DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[ref] = visited
		return nil
	}

	for _, t := range tasks {
		if err := visit(t.Ref); err != nil {
			return err
		}
	}

	return nil
}
