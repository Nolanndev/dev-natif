# dev-natif — API native Go pour Docker Engine

API REST native (Go) qui pilote le **Docker Engine** pour offrir une alternative
**augmentée** à Docker-Compose : modélisation, déploiement, supervision et gestion
du cycle de vie d'infrastructures logicielles multi-conteneurs.

L'idée directrice : séparer **la description** (un *Projet*, abstrait et
réutilisable, ≈ un fichier `docker-compose.yml`) de **son instanciation** (un
*Déploiement*, concret, ≈ le résultat de `docker-compose up`, portant les valeurs
spécifiques à l'environnement).

> Projet = description de l'infrastructure · Déploiement = instanciation sur un serveur Docker.

---

## Sommaire

- [Fonctionnalités](#fonctionnalités)
- [Démarrage rapide](#démarrage-rapide)
- [Configuration](#configuration)
- [Exemple complet (WordPress-like)](#exemple-complet)
- [Aperçu de l'API](#aperçu-de-lapi)
- [Architecture](#architecture)
- [Documentation détaillée](#documentation-détaillée)
- [Périmètre & roadmap](#périmètre--roadmap)

---

## Fonctionnalités

**Phase 1 (MVP) — implémentée et vérifiée**

- Interaction avec le Docker Engine **local** (via socket).
- **Description de projets** : services, volumes, variables d'environnement,
  mappings de ports, dépendances entre services (`depends_on`).
- **Déploiement** d'un projet avec **overrides** spécifiques (env, ports) — un
  même projet est réutilisable sur plusieurs déploiements.
- **Images** : `pull` et `build`.
- **Supervision** : état agrégé d'un déploiement
  (`running` / `partially-running` / `not-running`), calculé en direct à partir
  du Docker Engine via des labels de gestion.
- **Cycle de vie** : `up` (ordre topologique des dépendances), `down`, suppression.
- **Scaling** (replicas) — déjà fonctionnel (jalon Phase 2).
- **Journalisation** structurée (slog/JSON) + `X-Request-ID` par requête.
- Crochet de **sécurité** par clé d'API (`X-API-Key`, désactivé par défaut).

**Stack** : Go 1.25 · [Gin](https://github.com/gin-gonic/gin) · SDK Docker officiel ·
SQLite pur-Go ([`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite),
sans CGO) · image finale **~35 Mo**.

---

## Démarrage rapide

Prérequis : **Docker** (le moteur cible et l'outil de build). Aucun toolchain Go
local requis — tout est compilé dans l'image.

```bash
# Build + run (API + volume de données), socket Docker monté
docker compose up --build -d

# Vérifier
curl -s localhost:8080/healthz   # {"status":"ok"}
curl -s localhost:8080/readyz    # {"docker_engine":"ok","status":"ready"}
```

Ou directement avec Docker :

```bash
docker build -t dev-natif-api:latest .
docker run -d --name dev-natif-api -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v natif-data:/data \
  dev-natif-api:latest
```

> Le montage `/var/run/docker.sock` donne à l'API l'accès au Docker Engine de
> l'hôte. Sur Docker Desktop (macOS/Windows) ce chemin est pris en charge
> automatiquement.

Cibles `make` utiles : `make up`, `make down`, `make logs`, `make docker-build`.

---

## Configuration

Toute la configuration passe par variables d'environnement (valeurs par défaut
pensées pour le conteneur) :

| Variable            | Défaut                          | Rôle                                                        |
|---------------------|---------------------------------|------------------------------------------------------------|
| `NATIF_PORT`        | `8080`                          | Port d'écoute HTTP.                                         |
| `NATIF_DB_PATH`     | `/data/natif.db`                | Chemin du fichier SQLite (sur volume persistant).          |
| `NATIF_DOCKER_HOST` | *(vide → env/socket par défaut)*| Endpoint du Docker Engine (`unix://…` ou `tcp://…`).        |
| `NATIF_LOG_LEVEL`   | `info`                          | `debug` \| `info` \| `warn` \| `error`.                    |
| `NATIF_API_KEY`     | *(vide → auth désactivée)*       | Si défini, exige l'en-tête `X-API-Key` sur `/api/v1/*`.    |

---

## Exemple complet

Déployer un service nginx avec un **port** et une **variable d'environnement**
laissés *variables* dans le projet, puis fixés au moment du déploiement.

```bash
B=http://localhost:8080/api/v1

# 1) Projet
PID=$(curl -s -X POST $B/projects -d '{"name":"demo"}' | jq -r .id)

# 2) Volume
VID=$(curl -s -X POST $B/projects/$PID/volumes -d '{"name":"html"}' | jq -r .id)

# 3) Service (port 80 variable, env DEMO variable, montage du volume)
SID=$(curl -s -X POST $B/projects/$PID/services -d "{
  \"name\":\"web\",\"image\":\"nginx:alpine\",
  \"ports\":[{\"container_port\":80,\"is_variable\":true}],
  \"env\":[{\"key\":\"DEMO\",\"value\":\"default\",\"is_variable\":true}],
  \"mounts\":[{\"volume_id\":\"$VID\",\"target\":\"/usr/share/nginx/html\"}]
}" | jq -r .id)

# 4) Déploiement avec overrides (port -> 8899, env -> overridden)
DID=$(curl -s -X POST $B/projects/$PID/deployments -d "{
  \"name\":\"prod\",
  \"overrides\":[
    {\"kind\":\"port\",\"target_ref\":\"$SID\",\"key\":\"80/tcp\",\"value\":\"8899\"},
    {\"kind\":\"env\",\"target_ref\":\"$SID\",\"key\":\"DEMO\",\"value\":\"overridden\"}
  ]
}" | jq -r .id)

# 5) Instancier puis superviser
curl -s -X POST $B/deployments/$DID/up
curl -s $B/deployments/$DID/status      # -> "running"
curl -s localhost:8899                  # nginx répond (HTTP 200)

# 6) Arrêter / supprimer
curl -s -X POST $B/deployments/$DID/down
curl -s -X DELETE $B/deployments/$DID
curl -s -X DELETE $B/projects/$PID
```

---

## Aperçu de l'API

Base : `/api/v1`. Référence complète : [`docs/API.md`](docs/API.md) et la spec
OpenAPI [`api/openapi.yaml`](api/openapi.yaml).

| Domaine      | Endpoints |
|--------------|-----------|
| Santé        | `GET /healthz` · `GET /readyz` |
| Projets      | `POST/GET /projects` · `GET/PUT/DELETE /projects/:id` |
| Services     | `POST/GET /projects/:id/services` · `PUT/DELETE /projects/:id/services/:sid` |
| Volumes      | `POST/GET /projects/:id/volumes` · `DELETE /projects/:id/volumes/:vid` |
| Déploiements | `POST /projects/:id/deployments` · `GET /deployments` · `GET/DELETE /deployments/:id` · `POST /deployments/:id/up` · `POST /deployments/:id/down` · `GET /deployments/:id/status` |
| Images       | `POST /images/pull` · `POST /images/build` |
| Serveurs     | `GET /servers` · `GET /servers/:id` |

---

## Architecture

Architecture **hexagonale** (ports & adapters). Le `domain` ne dépend de rien ;
les adaptateurs (SQLite, Docker SDK, Gin) dépendent du `domain` via des
interfaces. Détails : [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

```
cmd/api            → composition root (câblage, serveur HTTP)
internal/domain    → entités + PORTS (interfaces) : repos & DockerEngine
internal/store     → adapter SQLite (modernc.org/sqlite)
internal/docker    → adapter Docker Engine (SDK officiel)
internal/service   → use-cases (overrides, tri topologique, up/down, statut)
internal/http      → couche Gin (router, middleware, handlers, DTO)
internal/config    → configuration par environnement
internal/logging   → logger structuré slog
```

---

## Documentation détaillée

- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — couches, flux d'une requête, extensibilité P2/P3.
- [`docs/API.md`](docs/API.md) — référence exhaustive des endpoints (exemples `curl`, codes d'erreur).
- [`docs/DATA-MODEL.md`](docs/DATA-MODEL.md) — schéma relationnel, ERD, conventions.
- [`docs/ROADMAP.md`](docs/ROADMAP.md) — couverture du sujet et phases.
- [`api/openapi.yaml`](api/openapi.yaml) — spécification OpenAPI 3.0.

---

## Périmètre & roadmap

- **Phase 1 (MVP)** : engine local, projets, déploiements, état, services/conteneurs,
  images, volumes, variables d'environnement. ✅ **Fait**.
- **Phase 2** : multi-engine + scaling. *Scaling déjà opérationnel* ; l'abstraction
  `Server` + `DockerEngine` par serveur prépare le multi-engine.
- **Phase 3** : réseaux, secrets, labels métier (ex. Traefik). *Crochets en place*
  (entités/labels), à compléter.

Voir [`docs/ROADMAP.md`](docs/ROADMAP.md).
