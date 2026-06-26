# Référence de l'API

Base URL : `http://<host>:8080` · Préfixe versionné : `/api/v1`.
Format : JSON (`Content-Type: application/json`).
Spécification machine : [`api/openapi.yaml`](../api/openapi.yaml).

## Authentification

Quand l'auth est activée (`NATIF_AUTH_ENABLED=true`, défaut), **toutes** les routes
`/api/v1/*` sauf `POST /auth/login` exigent un en-tête :

```
Authorization: Bearer <token>
```

Cycle de vie du token (JWT HS256) :

1. `POST /api/v1/auth/login` avec `{username, password}` → renvoie un `token` et sa
   date d'expiration (`expires_at`). Identifiants par défaut : `admin` / `admin`.
2. Le token expire après `NATIF_TOKEN_TTL` (défaut `1h`).
3. `POST /api/v1/auth/refresh` (avec un token encore valide) → nouveau token, expiry
   prolongée. La console web renouvelle automatiquement avant l'échéance.

### `POST /api/v1/auth/login`
```json
{ "username": "admin", "password": "admin" }
```
→ `200 { "token": "...", "token_type": "Bearer", "expires_at": "...", "username": "admin" }`
· `401` si identifiants invalides.

### `POST /api/v1/auth/refresh`
En-tête `Authorization: Bearer <token>` requis (token valide). → même forme que login. `401` si expiré/invalide.

### `GET /api/v1/auth/me`
→ `200 { "username": "...", "expires_at": "..." }`.

## Conventions

- **Corrélation** : chaque réponse renvoie `X-Request-ID` (repris ou généré).
- **Erreurs** : enveloppe uniforme `{"error":"message"}`.

### Codes d'erreur

| Cause (erreur domaine) | HTTP |
|------------------------|------|
| Validation | `400 Bad Request` |
| Auth manquante/invalide/expirée | `401 Unauthorized` |
| Ressource introuvable | `404 Not Found` |
| Conflit (unicité) | `409 Conflict` |
| Cycle de dépendances | `422 Unprocessable Entity` |
| Erreur du Docker Engine (daemon) | `502 Bad Gateway` |
| Autre | `500 Internal Server Error` |

> Les erreurs `502` (daemon Docker) lors d'un `up`/`down` sont aussi **persistées
> en événements** (voir [Historique & événements](#historique--événements)) et
> restent consultables, pas seulement renvoyées dans la réponse.

---

## Santé

### `GET /healthz`
Liveness. → `200 {"status":"ok"}` (public, sans token).

### `GET /readyz`
Readiness + joignabilité du Docker Engine. → `200 {"status":"ready","docker_engine":"ok"}`
ou `503` si le moteur est injoignable (public).

---

## Projets

### `POST /api/v1/projects`
```json
{ "name": "demo", "description": "optionnel" }
```
→ `201` projet créé. Erreurs : `400` (nom requis), `409` (nom déjà pris).

### `GET /api/v1/projects`
Liste les projets (sans enfants). → `200 [ {…} ]`

### `GET /api/v1/projects/:id`
Détail **complet** (services + env/ports/mounts/deps, volumes). → `200` / `404`.

### `PUT /api/v1/projects/:id`
Met à jour nom/description. → `200` / `400` / `404`.

### `DELETE /api/v1/projects/:id`
Supprime le projet (cascade services/volumes/déploiements) **et détruit les
conteneurs et réseaux** encore actifs de ses déploiements (aucun orphelin). → `204` / `404`.

---

## Services

### `POST /api/v1/projects/:id/services`
`image` **ou** `build_context` requis.

```json
{
  "name": "web",
  "image": "nginx:alpine",
  "build_context": "", "build_dockerfile": "Dockerfile",
  "command": ["nginx","-g","daemon off;"],
  "restart_policy": "unless-stopped",
  "replicas": 1,
  "env":   [ { "key": "DEMO", "value": "default", "is_variable": true } ],
  "ports": [ { "container_port": 80, "host_port": 0, "protocol": "tcp", "is_variable": true } ],
  "mounts":[ { "volume_id": "<id>", "target": "/usr/share/nginx/html", "read_only": false } ],
  "depends_on": [ "<service_id>" ]
}
```
→ `201`. Erreurs : `400` (nom requis, ou ni image ni build), `404` (projet), `409`.

> `is_variable: true` ⇒ valeur à fournir au déploiement via un *override*.
> `depends_on` ⇒ ordre de démarrage (tri topologique) à l'`up`.

### `GET /api/v1/projects/:id/services` · `PUT …/services/:sid` · `DELETE …/services/:sid`
Liste / remplace / supprime. → `200` / `204` / `404`.

---

## Volumes

### `POST /api/v1/projects/:id/volumes`
```json
{ "name": "html", "driver": "local" }
```
### `GET /api/v1/projects/:id/volumes` · `DELETE …/volumes/:vid`

---

## Déploiements

### `POST /api/v1/projects/:id/deployments`
```json
{
  "name": "prod", "server_id": "local",
  "overrides": [
    { "kind": "port", "target_ref": "<service_id>", "key": "80/tcp", "value": "8899" },
    { "kind": "env",  "target_ref": "<service_id>", "key": "DEMO",   "value": "overridden" }
  ]
}
```
`kind` ∈ { `env`, `port` } ; pour un port, `key` = `"<containerPort>/<proto>"`.
→ `201` (statut `pending`). Enregistre un événement `deployment.created`.

### `GET /api/v1/deployments`
Liste globale (sans enfants).

### `GET /api/v1/projects/:id/deployments`
**Historique des déploiements d'un projet** (du plus récent au plus ancien).

### `GET /api/v1/deployments/:id`
Détail (overrides + conteneurs suivis). → `200` / `404`.

### `POST /api/v1/deployments/:id/up`
Instancie : crée un **réseau dédié** au déploiement, les volumes, puis crée/démarre
les conteneurs dans l'ordre des dépendances en appliquant les overrides. Tous les
conteneurs rejoignent le réseau avec un **alias = nom de service** (résolution DNS
inter-services et round-robin entre replicas). → `200`. Erreurs : `404`, `422`
(cycle), `502` (daemon) ; en cas d'échec, un événement `deployment.failed` (niveau
`error`) est enregistré avec le message du daemon.

### `POST /api/v1/deployments/:id/down`
Arrête/supprime les conteneurs et le réseau (volumes conservés). Événement `deployment.down`.

### `GET /api/v1/deployments/:id/status`
État agrégé **en direct** + conteneurs rafraîchis.
```json
{ "status": "running", "containers": [ { "name": "…", "state": "running", "health": "none", "docker_container_id": "…", "service_id": "…" } ] }
```
`status` ∈ { `pending`, `running`, `partially-running`, `not-running`, `failed` }.

### `DELETE /api/v1/deployments/:id`
`down` puis suppression. Événement `deployment.deleted`. → `204` / `404`.

---

## Logs des conteneurs

### `GET /api/v1/deployments/:id/containers/:cid/logs?tail=200`
`cid` = ID Docker du conteneur (cf. `status`). `tail` = nombre de lignes (défaut 200).
→ `200 { "logs": "…stdout+stderr horodatés…" }` / `404` / `502`.

---

## Historique & événements

Les événements tracent le cycle de vie des déploiements et les **erreurs du daemon
Docker**. Types : `deployment.created|up|down|failed|deleted`, `image.pull|build`,
`error.docker`. Niveaux : `info` | `warn` | `error`. Triés du plus récent au plus
ancien ; paramètre `?limit=` (défaut 100).

| Endpoint | Portée |
|----------|--------|
| `GET /api/v1/projects/:id/events` | événements d'un projet |
| `GET /api/v1/deployments/:id/events` | historique d'un déploiement |
| `GET /api/v1/events` | flux global (activité + erreurs) |

```json
[ { "id":"…","created_at":"…","level":"error","type":"deployment.failed",
    "project_id":"…","deployment_id":"…","message":"… No such image: …","details":"" } ]
```

> **Rétention** : les événements de plus de `NATIF_RETENTION_DAYS` jours (défaut 30)
> sont purgés automatiquement (au démarrage puis toutes les 6 h). Les logs de
> conteneurs ne sont **pas** stockés en base : ils sont lus en direct depuis le
> Docker Engine, donc rien à purger côté API.

---

## Images

### `GET /api/v1/images`
Liste les images présentes sur l'Engine.
```json
[ { "id":"sha256:…", "tags":["nginx:alpine"], "size":12345678, "created":1700000000 } ]
```

### `POST /api/v1/images/pull`
```json
{ "ref": "nginx:alpine", "auth": "" }
```
`auth` = jeton `X-Registry-Auth` base64 (registries privés). → `200` / `400` / `502`.

### `POST /api/v1/images/build`
```json
{ "context_dir": "/chemin/accessible/au/moteur", "dockerfile": "Dockerfile", "tag": "monimage:latest" }
```
> `context_dir` est lu **sur le système de fichiers du processus API** (le conteneur
> de l'API), pas sur la machine cliente.

---

## Serveurs

### `GET /api/v1/servers` · `GET /api/v1/servers/:id`
Cibles Docker Engine (un seul `local` en MVP).
