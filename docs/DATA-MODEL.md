# Modèle de données

Stockage **relationnel** sur **SQLite** (fichier unique sur volume persistant,
`/data/natif.db`). Driver pur-Go `modernc.org/sqlite`. Le schéma de référence est
[`migrations/0001_init.sql`](../migrations/0001_init.sql) (copié et embarqué dans
[`internal/store/schema.sql`](../internal/store/schema.sql), appliqué de façon
idempotente au démarrage : `CREATE TABLE IF NOT EXISTS`, `INSERT OR IGNORE`).

## Diagramme entité-relation

```
servers
  └──< deployments >── projects
                          │
        ┌─────────────────┼───────────────────┐
        │                 │                    │
     volumes           services            deployments
        │                 │                    │
        │        ┌────────┼─────────┐    ┌─────┴──────┐
        │     service_  service_  service_ deployment_  containers
        │      env      ports     mounts   overrides
        │                 │          │
        │              service_deps  │
        └──────────────────(volume)──┘   (service_mounts.volume_id → volumes.id)

Légende :  A ──< B  =  un A possède plusieurs B (1..N)
```

## Tables

| Table | Rôle | Clés étrangères (ON DELETE) |
|-------|------|------------------------------|
| `servers` | Cibles Docker Engine (un seul `local` seedé). | — |
| `projects` | Description abstraite d'infrastructure. | — |
| `volumes` | Volumes persistants d'un projet. | `project_id → projects` (CASCADE) |
| `services` | Services d'un projet. | `project_id → projects` (CASCADE) |
| `service_env` | Variables d'environnement d'un service. | `service_id → services` (CASCADE) |
| `service_ports` | Mappings de ports d'un service. | `service_id → services` (CASCADE) |
| `service_mounts` | Montages volume↔service. | `service_id → services`, `volume_id → volumes` (CASCADE) |
| `service_deps` | Dépendances `depends_on` (graphe). | `service_id`, `depends_on_id → services` (CASCADE) |
| `deployments` | Instanciation d'un projet sur un serveur. | `project_id → projects` (CASCADE), `server_id → servers` |
| `deployment_overrides` | Valeurs spécifiques au déploiement (env/port). | `deployment_id → deployments` (CASCADE) |
| `containers` | Suivi runtime des conteneurs instanciés. | `deployment_id → deployments`, `service_id → services` (CASCADE) |

## Conventions de mapping (Go ↔ SQL)

- **Identifiants** : `TEXT` (UUID v4, générés côté `store` si absents).
- **`services.command`** (`[]string`) : sérialisé en **JSON** dans une colonne `TEXT`.
- **Booléens** (`is_variable`, `read_only`) : `INTEGER` 0/1.
- **Horodatages** (`created_at`, `updated_at`) : `TIMESTAMP` (UTC), posés à la création/màj.
- **Contraintes d'unicité** : `projects.name`, `services(project_id,name)`,
  `volumes(project_id,name)`, `service_env(service_id,key)` → mappées sur
  `domain.ErrConflict` (HTTP 409).
- **Champ variable** (`*.is_variable`) : marque ce qui **doit** être fourni par un
  `deployment_overrides` à l'instanciation (env, port d'hôte).

## Index

- `idx_services_project (services.project_id)`
- `idx_deployments_project (deployments.project_id)`
- `idx_containers_deploy (containers.deployment_id)`

## Stratégie de chargement

- `GetProject` **hydrate** services (+ env/ports/mounts/deps) et volumes (vue détaillée).
- `ListProjects` renvoie des projets **sans** enfants (évite le N+1 sur la liste).
- `GetDeployment` **hydrate** overrides et conteneurs ; `ListDeployments` sans enfants.
- `SaveContainers` **remplace** l'ensemble des conteneurs suivis d'un déploiement
  (delete + insert), maintenant la cohérence avec l'état réel du moteur.
