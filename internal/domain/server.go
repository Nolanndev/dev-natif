package domain

// Server is a Docker Engine target (local or remote). The MVP manages a single
// local server, but deployments already reference a ServerID so Phase 2
// (multi-engine) requires no schema or interface change.
type Server struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Host   string `json:"host"`   // e.g. "unix:///var/run/docker.sock" or "tcp://host:2375"
	Status string `json:"status"` // "reachable" | "unreachable"
}

// LocalServerID is the well-known identifier of the single local engine in the MVP.
const LocalServerID = "local"
