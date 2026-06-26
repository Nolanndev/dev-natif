# Référence de l'API

Base URL : `http://<host>:8080` · Préfixe versionné : `/api/v1`.
Format : JSON (`Content-Type: application/json`).
Spécification machine : [`api/openapi.yaml`](../api/openapi.yaml).

## Conventions

- **Authentification** : si `NATIF_API_KEY` est défini, toute requête vers
  `/api/v1/*` doit porter l'en-tête `X-API-Key: <clé>` (sinon `401`).
  Les endpoints de santé (`/healthz`, `/readyz`) ne sont jamais protégés.
- **Corrélation** : chaque réponse renvoie l'en-tête `X-Request-ID` (repris de la
  requête s'il est fourni, sinon généré).
- **Erreurs** : enveloppe uniforme `{"error":"message"}`.

### Codes d'erreur

| Cause (erreur domaine) | HTTP |
|------------------------|------|
| Validation (champ requis, etc.) | `400 Bad Request` |
| Authentification manquante/invalide | `401 Unauthorized` |
| Ressource introuvable | `404 Not Found` |
| Conflit (unicité) | `409 Conflict` |
| Cycle de dépendances | `422 Unprocessable Entity` |
| Erreur du Docker Engine | `502 Bad Gateway` |
| Autre | `500 Internal Server Error` |

---

## Santé

### `GET /healthz`
Liveness. → `200 {"status":"ok"}`

### `GET /readyz`
Readiness, inclut la joignabilité du Docker Engine.
→ `200 {"status":"ready","docker_engine":"ok"}` ou `503` si le moteur est injoignable.

---

## Projets

### `POST /api/v1/projects`
Crée un projet.

```json
{ "name": "demo", "description": "optionnel" }
```
→ `201` projet créé (`id`, `name`, `description`, `created_at`, `updated_at`).
Erreurs : `400` (nom requis), `409` (nom déjà pris).

### `GET /api/v1/projects`
Liste les projets (sans enfants). → `200 [ {…} ]`

### `GET /api/v1/projects/:id`
Détail **complet** (services + env/ports/mounts/deps, volumes). → `200` / `404`.

### `PUT /api/v1/projects/:id`
Met à jour nom/description. Corps identique à la création. → `200` / `400` / `404`.

### `DELETE /api/v1/projects/:id`
Supprime le projet et, en cascade, ses services/volumes/déploiements. → `204` / `404`.

---

## Services

### `POST /api/v1/projects/:id/services`
Ajoute un service au projet. `image` **ou** `build_context` est requis.

```json
{
  "name": "web",
  "image": "nginx:alpine",
  "build_context": "",
  "build_dockerfile": "Dockerfile",
  "command": ["nginx","-g","daemon off;"],
  "restart_policy": "unless-stopped",
  "replicas": 1,
  "env":   [ { "key": "DEMO", "value": "default", "is_variable": true } ],
  "ports": [ { "container_port": 80, "host_port": 0, "protocol": "tcp", "is_variable": true } ],
  "mounts":[ { "volume_id": "<id>", "target": "/usr/share/nginx/html", "read_only": false } ],
  "depends_on": [ "<service_id>" ]
}
```
→ `201` service créé. Erreurs : `400` (nom requis, ou ni image ni build), `404` (projet), `409`.

> `is_variable: true` sur un env ou un port signale une valeur à fournir au
> moment du déploiement via un *override*.

### `GET /api/v1/projects/:id/services`
Liste les services du projet (hydratés). → `200`.

### `PUT /api/v1/projects/:id/services/:sid`
Remplace la définition du service. → `200` / `400` / `404`.

### `DELETE /api/v1/projects/:id/services/:sid`
Supprime le service. → `204` / `404`.

---

## Volumes

### `POST /api/v1/projects/:id/volumes`
```json
{ "name": "html", "driver": "local" }
```
→ `201`. Erreurs : `400`, `404`, `409`.

### `GET /api/v1/projects/:id/volumes`
Liste les volumes du projet. → `200`.

### `DELETE /api/v1/projects/:id/volumes/:vid`
→ `204` / `404`.

---

## Déploiements

### `POST /api/v1/projects/:id/deployments`
Crée un déploiement (non encore instancié) avec ses overrides.

```json
{
  "name": "prod",
  "server_id": "local",
  "overrides": [
    { "kind": "port", "target_ref": "<service_id>", "key": "80/tcp", "value": "8899" },
    { "kind": "env",  "target_ref": "<service_id>", "key": "DEMO",   "value": "overridden" }
  ]
}
```
- `server_id` optionnel → serveur par défaut (`local`).
- `kind` ∈ { `env`, `port` }. Pour un port, `key` = `"<containerPort>/<proto>"`.

→ `201` déploiement (statut initial `pending`). Erreurs : `400`, `404` (projet/serveur).

### `GET /api/v1/deployments`
Liste les déploiements (sans enfants). → `200`.

### `GET /api/v1/deployments/:id`
Détail (overrides + conteneurs suivis). → `200` / `404`.

### `POST /api/v1/deployments/:id/up`
Instancie le projet : crée les volumes, puis crée et démarre les conteneurs dans
l'ordre des dépendances, applique les overrides. → `200` déploiement à jour.
Erreurs : `404`, `422` (cycle de dépendances), `502` (erreur moteur).

### `POST /api/v1/deployments/:id/down`
Arrête et supprime les conteneurs du déploiement (volumes conservés).
→ `200 {"status":"down"}` / `404` / `502`.

### `GET /api/v1/deployments/:id/status`
État agrégé **en direct** + conteneurs rafraîchis.
```json
{ "status": "running", "containers": [ { "name": "…", "state": "running", "health": "none", … } ] }
```
`status` ∈ { `pending`, `running`, `partially-running`, `not-running`, `failed` }.

### `DELETE /api/v1/deployments/:id`
`down` best-effort puis suppression de l'enregistrement. → `204` / `404`.

---

## Images

### `POST /api/v1/images/pull`
```json
{ "ref": "nginx:alpine", "auth": "" }
```
`auth` = jeton `X-Registry-Auth` base64 optionnel (registries privés).
→ `200 {"status":"pulled","ref":"…"}` / `400` / `502`.

### `POST /api/v1/images/build`
```json
{ "context_dir": "/chemin/accessible/au/moteur", "dockerfile": "Dockerfile", "tag": "monimage:latest" }
```
→ `200 {"status":"built","tag":"…"}` / `400` / `502`.

> `context_dir` est résolu **côté Docker Engine**.

---

## Serveurs

### `GET /api/v1/servers`
Liste les cibles Docker Engine. → `200`.

### `GET /api/v1/servers/:id`
→ `200` / `404`. (Un seul serveur `local` en MVP.)
