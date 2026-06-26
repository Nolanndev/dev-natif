package domain

import "time"

// Event levels.
const (
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// Event types (stable identifiers for filtering/UI).
const (
	EvtDeploymentCreated = "deployment.created"
	EvtDeploymentUp      = "deployment.up"
	EvtDeploymentDown    = "deployment.down"
	EvtDeploymentFailed  = "deployment.failed"
	EvtDeploymentDeleted = "deployment.deleted"
	EvtImagePull         = "image.pull"
	EvtImageBuild        = "image.build"
	EvtDockerError       = "error.docker"
)

// Event is an audit/history record: a deployment lifecycle step or an error
// (notably Docker daemon errors), kept so they can be reviewed in the UI rather
// than only flashing by. Events are purged after a retention window.
type Event struct {
	ID           string    `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	Level        string    `json:"level"` // info | warn | error
	Type         string    `json:"type"`
	ProjectID    string    `json:"project_id,omitempty"`
	DeploymentID string    `json:"deployment_id,omitempty"`
	Message      string    `json:"message"`
	Details      string    `json:"details,omitempty"`
}

// EventFilter narrows an event query. Empty fields are ignored. Limit defaults
// to a sane cap when <= 0.
type EventFilter struct {
	ProjectID    string
	DeploymentID string
	Limit        int
}
