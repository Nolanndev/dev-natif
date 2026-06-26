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

- Interaction avec le Docker Engine **local** (via socket).
- **Description de projets** : services, volumes, variables d'environnement,
  mappings de ports, dépendances entre services (`depends_on`).
- **Déploiement** d'un projet avec **overrides** spécifiques (env, ports) — un
  même projet est réutilisable sur plusieurs déploiements.
- **Réseau par déploiement** : tous les conteneurs d'un déploiement (services +
  replicas) sont reliés sur un réseau dédié avec **résolution DNS par nom de
  service**, exactement comme le réseau par défaut de docker-compose (`web` joint
  `db` ; les replicas partagent l'alias).
- **Images** : `pull`, `build` et **listing** des images présentes sur l'Engine.
- **Supervision** : état agrégé d'un déploiement
  (`running` / `partially-running` / `not-running`), calculé en direct à partir
  du Docker Engine via des labels de gestion.
- **Historique & événements** : chaque déploiement garde l'historique de son
  cycle de vie ; les **erreurs du daemon Docker** sont **persistées et affichées**
  dans une interface claire (plus seulement des messages flash).
- **Logs des conteneurs** : consultation des logs (stdout/stderr) depuis l'UI/API.
- **Rétention** : les événements/historique de plus de **30 jours** sont purgés
  automatiquement (configurable) pour ne pas surcharger la base.
- **Cycle de vie** : `up` (ordre topologique des dépendances), `down`, suppression
  (avec nettoyage des conteneurs/réseaux, sans orphelins).
- **Scaling** (replicas).
- **Authentification** par **token JWT** : login, expiration, renouvellement.
- **Journalisation** structurée (slog/JSON) + `X-Request-ID` par requête.
- **Console web** intégrée couvrant tout le projet.

**Stack** : Go 1.25 · [Gin](https://github.com/gin-gonic/gin) · SDK Docker officiel ·
SQLite pur-Go ([`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite),
sans CGO) · JWT ([`golang-jwt`](https://github.com/golang-jwt/jwt)) ·
front vanilla JS embarqué · image finale **~35 Mo**.

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

Puis **ouvre la console web** : <http://localhost:8080/> — une interface graphique qui
couvre tout le projet (projets, services, volumes, déploiements, images, serveurs,
supervision en direct). Voir [Console web](#console-web).

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
| `NATIF_AUTH_ENABLED`| `true`                          | Active l'authentification par token sur `/api/v1/*`.       |
| `NATIF_AUTH_USERNAME`| `admin`                        | Identifiant de connexion.                                  |
| `NATIF_AUTH_PASSWORD`| `admin`                        | Mot de passe (à changer en production).                    |
| `NATIF_JWT_SECRET`  | *(généré si vide)*              | Secret de signature HS256 (à fixer pour des tokens stables).|
| `NATIF_TOKEN_TTL`   | `1h`                            | Durée de vie du token (ex. `15m`, `8h`).                   |
| `NATIF_RETENTION_DAYS`| `30`                          | Rétention des événements/historique en jours (0 = jamais purger).|

> **Authentification.** Quand activée (par défaut), toutes les routes `/api/v1/*`
> (sauf `/auth/login`) exigent un en-tête `Authorization: Bearer <token>`. Obtenez
> un token via `POST /api/v1/auth/login` (`admin`/`admin` par défaut) ; il expire
> selon `NATIF_TOKEN_TTL` et se renouvelle via `POST /api/v1/auth/refresh`. La
> console web gère tout cela (écran de connexion, renouvellement automatique).
> Pour désactiver en dev : `NATIF_AUTH_ENABLED=false`.

---

## Exemple complet

Déployer un service nginx avec un **port** et une **variable d'environnement**
laissés *variables* dans le projet, puis fixés au moment du déploiement.

```bash
B=http://localhost:8080/api/v1

# 0) Authentification (auth activée par défaut) → récupérer un token
TOKEN=$(curl -s -X POST $B/auth/login -d '{"username":"admin","password":"admin"}' | jq -r .token)
auth=(-H "Authorization: Bearer $TOKEN")   # à passer à chaque appel

# 1) Projet
PID=$(curl -s "${auth[@]}" -X POST $B/projects -d '{"name":"demo"}' | jq -r .id)

# 2) Volume
VID=$(curl -s "${auth[@]}" -X POST $B/projects/$PID/volumes -d '{"name":"html"}' | jq -r .id)

# 3) Service (port 80 variable, env DEMO variable, montage du volume)
SID=$(curl -s "${auth[@]}" -X POST $B/projects/$PID/services -d "{
  \"name\":\"web\",\"image\":\"nginx:alpine\",
  \"ports\":[{\"container_port\":80,\"is_variable\":true}],
  \"env\":[{\"key\":\"DEMO\",\"value\":\"default\",\"is_variable\":true}],
  \"mounts\":[{\"volume_id\":\"$VID\",\"target\":\"/usr/share/nginx/html\"}]
}" | jq -r .id)

# 4) Déploiement avec overrides (port -> 8899, env -> overridden)
DID=$(curl -s "${auth[@]}" -X POST $B/projects/$PID/deployments -d "{
  \"name\":\"prod\",
  \"overrides\":[
    {\"kind\":\"port\",\"target_ref\":\"$SID\",\"key\":\"80/tcp\",\"value\":\"8899\"},
    {\"kind\":\"env\",\"target_ref\":\"$SID\",\"key\":\"DEMO\",\"value\":\"overridden\"}
  ]
}" | jq -r .id)

# 5) Instancier puis superviser
curl -s "${auth[@]}" -X POST $B/deployments/$DID/up
curl -s "${auth[@]}" $B/deployments/$DID/status   # -> "running"
curl -s localhost:8899                            # nginx répond (HTTP 200)

# 6) Arrêter / supprimer
curl -s "${auth[@]}" -X POST $B/deployments/$DID/down
curl -s "${auth[@]}" -X DELETE $B/deployments/$DID
curl -s "${auth[@]}" -X DELETE $B/projects/$PID
```

---

## Console web

Une **interface graphique** est embarquée dans le binaire (via `go:embed`) et servie
en *same-origin* sur <http://localhost:8080/> — **aucun conteneur ni build Node
supplémentaire**, l'image reste ~35 Mo.

Elle couvre l'intégralité du projet :

- **Projets** : créer, lister, ouvrir le détail, supprimer.
- **Services** : formulaire riche (image/build, commande, restart, replicas,
  variables d'env avec marquage *variable*, ports avec marquage *variable*,
  montages de volumes, dépendances `depends_on`).
- **Volumes** : ajout/suppression par projet.
- **Déploiements** : création avec **overrides** (les variables du projet sont
  proposées automatiquement), `up` / `down`, détail avec **supervision en direct**
  (badge d'état, conteneurs, santé, rafraîchissement automatique), **historique**
  par projet, **panneau Activité & erreurs** (les erreurs daemon y restent
  lisibles), et **visualiseur de logs** par conteneur.
- **Images** : `pull`, `build`, et **liste** des images présentes.
- **Serveurs** : liste de la cible Docker Engine.
- **Connexion** : écran de login (token JWT), renouvellement automatique avant
  expiration, déconnexion ; indicateur de santé du Docker Engine.

Stack front : **vanilla JS + CSS**, zéro dépendance, zéro build. Thème sombre,
URLs par hash (rafraîchissement et bouton retour pris en charge), responsive.

## Aperçu de l'API

Base : `/api/v1`. Référence complète : [`docs/API.md`](docs/API.md) et la spec
OpenAPI [`api/openapi.yaml`](api/openapi.yaml).

| Domaine      | Endpoints |
|--------------|-----------|
| Santé        | `GET /healthz` · `GET /readyz` |
| Auth         | `POST /auth/login` · `POST /auth/refresh` · `GET /auth/me` |
| Projets      | `POST/GET /projects` · `GET/PUT/DELETE /projects/:id` |
| Services     | `POST/GET /projects/:id/services` · `PUT/DELETE /projects/:id/services/:sid` |
| Volumes      | `POST/GET /projects/:id/volumes` · `DELETE /projects/:id/volumes/:vid` |
| Déploiements | `POST /projects/:id/deployments` · `GET /projects/:id/deployments` · `GET /deployments` · `GET/DELETE /deployments/:id` · `POST /deployments/:id/up` · `POST /deployments/:id/down` · `GET /deployments/:id/status` |
| Logs         | `GET /deployments/:id/containers/:cid/logs` |
| Historique   | `GET /projects/:id/events` · `GET /deployments/:id/events` · `GET /events` |
| Images       | `GET /images` · `POST /images/pull` · `POST /images/build` |
| Serveurs     | `GET /servers` · `GET /servers/:id` |

> Toutes les routes `/api/v1/*` (sauf `/auth/login`) exigent `Authorization: Bearer <token>` quand l'auth est activée.

---

## Architecture

Architecture **hexagonale** (ports & adapters). Le `domain` ne dépend de rien ;
les adaptateurs (SQLite, Docker SDK, Gin) dépendent du `domain` via des
interfaces. Détails : [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

```
cmd/api            → composition root (câblage, serveur HTTP, purge rétention)
internal/domain    → entités + PORTS (interfaces) : repos, DockerEngine, events
internal/store     → adapter SQLite (modernc.org/sqlite) + repo événements
internal/docker    → adapter Docker Engine (SDK : conteneurs, images, volumes, réseaux, logs)
internal/service   → use-cases (overrides, tri topologique, up/down, statut, événements)
internal/auth      → tokens JWT (login, refresh, validation)
internal/http      → couche Gin (router, middleware, handlers, DTO) + UI embarquée
internal/http/web  → console web (SPA statique : index.html, app.js, styles.css)
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
