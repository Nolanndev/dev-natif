// Package service holds the use-case layer: it orchestrates the persistence and
// Docker Engine ports to fulfil the application's behaviour. It depends only on
// the interfaces declared in internal/domain.
package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/actigraph/dev-natif/internal/domain"
)

func validation(format string, args ...any) error {
	return fmt.Errorf("%w: %s", domain.ErrValidation, fmt.Sprintf(format, args...))
}

var invalidName = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

// sanitizeName makes an arbitrary string usable as a Docker object name.
func sanitizeName(parts ...string) string {
	joined := strings.Join(parts, "_")
	joined = strings.ToLower(joined)
	joined = invalidName.ReplaceAllString(joined, "-")
	joined = strings.Trim(joined, "-_.")
	if joined == "" {
		joined = "obj"
	}
	return joined
}

// topoSort orders services so that every dependency comes before the services
// that depend on it (Kahn's algorithm). Returns domain.ErrDependencyCyc on a
// cycle. Dependencies referencing unknown service IDs are ignored.
func topoSort(services []*domain.Service) ([]*domain.Service, error) {
	byID := make(map[string]*domain.Service, len(services))
	for _, s := range services {
		byID[s.ID] = s
	}

	indegree := make(map[string]int, len(services))
	adj := make(map[string][]string, len(services))
	for _, s := range services {
		if _, ok := indegree[s.ID]; !ok {
			indegree[s.ID] = 0
		}
		for _, dep := range s.DependsOn {
			if _, ok := byID[dep]; !ok {
				continue // unknown dependency, skip
			}
			adj[dep] = append(adj[dep], s.ID)
			indegree[s.ID]++
		}
	}

	// Seed queue with services that have no dependencies, preserving input order.
	var queue []string
	for _, s := range services {
		if indegree[s.ID] == 0 {
			queue = append(queue, s.ID)
		}
	}

	var ordered []*domain.Service
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		ordered = append(ordered, byID[id])
		for _, next := range adj[id] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(ordered) != len(services) {
		return nil, domain.ErrDependencyCyc
	}
	return ordered, nil
}

// portKey builds the override key for a port: "<containerPort>/<proto>".
func portKey(containerPort int, proto string) string {
	if proto == "" {
		proto = "tcp"
	}
	return fmt.Sprintf("%d/%s", containerPort, proto)
}
