-- dev-natif initial schema (SQLite).
-- Relational model: Project -> Service -> {env, ports, mounts, deps}; Project ->
-- Volume; Project -> Deployment -> {overrides, containers}. Servers are the
-- Docker Engine targets (single local row in the MVP).
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS servers (
    id     TEXT PRIMARY KEY,
    name   TEXT NOT NULL,
    host   TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'unknown'
);

CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMP NOT NULL,
    updated_at  TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS volumes (
    id         TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    driver     TEXT NOT NULL DEFAULT 'local',
    UNIQUE(project_id, name)
);

CREATE TABLE IF NOT EXISTS services (
    id               TEXT PRIMARY KEY,
    project_id       TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    image            TEXT NOT NULL DEFAULT '',
    build_context    TEXT NOT NULL DEFAULT '',
    build_dockerfile TEXT NOT NULL DEFAULT '',
    command          TEXT NOT NULL DEFAULT '',   -- JSON-encoded []string
    restart_policy   TEXT NOT NULL DEFAULT '',
    replicas         INTEGER NOT NULL DEFAULT 1,
    created_at       TIMESTAMP NOT NULL,
    UNIQUE(project_id, name)
);

CREATE TABLE IF NOT EXISTS service_env (
    id          TEXT PRIMARY KEY,
    service_id  TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    key         TEXT NOT NULL,
    value       TEXT NOT NULL DEFAULT '',
    is_variable INTEGER NOT NULL DEFAULT 0,
    UNIQUE(service_id, key)
);

CREATE TABLE IF NOT EXISTS service_ports (
    id             TEXT PRIMARY KEY,
    service_id     TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    container_port INTEGER NOT NULL,
    host_port      INTEGER NOT NULL DEFAULT 0,
    protocol       TEXT NOT NULL DEFAULT 'tcp',
    is_variable    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS service_mounts (
    id         TEXT PRIMARY KEY,
    service_id TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    volume_id  TEXT NOT NULL REFERENCES volumes(id) ON DELETE CASCADE,
    target     TEXT NOT NULL,
    read_only  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS service_deps (
    service_id    TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    depends_on_id TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    PRIMARY KEY (service_id, depends_on_id)
);

CREATE TABLE IF NOT EXISTS deployments (
    id         TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    server_id  TEXT NOT NULL REFERENCES servers(id),
    name       TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS deployment_overrides (
    id            TEXT PRIMARY KEY,
    deployment_id TEXT NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    kind          TEXT NOT NULL,        -- 'env' | 'port'
    target_ref    TEXT NOT NULL,        -- service ID
    key           TEXT NOT NULL,
    value         TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS containers (
    id                  TEXT PRIMARY KEY,
    deployment_id       TEXT NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    service_id          TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    docker_container_id TEXT NOT NULL DEFAULT '',
    name                TEXT NOT NULL DEFAULT '',
    state               TEXT NOT NULL DEFAULT '',
    health              TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_services_project   ON services(project_id);
CREATE INDEX IF NOT EXISTS idx_deployments_project ON deployments(project_id);
CREATE INDEX IF NOT EXISTS idx_containers_deploy  ON containers(deployment_id);

-- Seed the single local engine target used by the MVP.
INSERT OR IGNORE INTO servers(id, name, host, status)
VALUES ('local', 'local', 'unix:///var/run/docker.sock', 'unknown');
