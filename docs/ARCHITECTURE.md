# Architecture

## 1. Principes

Le projet suit une **architecture hexagonale** (ports & adapters) avec une
dépendance dirigée vers le centre :

```
            ┌─────────────────────────────────────────────┐
            │                  internal/http               │  (adapter entrant : Gin)
            └───────────────────────┬─────────────────────┘
                                    │ appelle
            ┌───────────────────────▼─────────────────────┐
            │                internal/service              │  (use-cases / logique métier)
            └───────────┬───────────────────────┬─────────┘
                        │ dépend de              │ dépend de
            ┌───────────▼─────────┐   ┌──────────▼──────────┐
            │   PORTS (domain)    │   │    PORTS (domain)   │
            │  *Repository        │   │   DockerEngine      │
            └───────────▲─────────┘   └──────────▲──────────┘
                        │ implémente             │ implémente
            ┌───────────┴─────────┐   ┌──────────┴──────────┐
            │   internal/store    │   │   internal/docker   │  (adapters sortants)
            │   (SQLite)          │   │   (Docker SDK)      │
            └─────────────────────┘   └─────────────────────┘
```

**Règle d'or** : `internal/domain` n'importe **aucune** dépendance d'infrastructure
(ni Gin, ni le SDK Docker, ni SQLite). Tout le reste dépend du domaine, jamais
l'inverse. Conséquence directe : la couche `service` et la couche `http` se codent
et se testent contre des **interfaces**, et les adaptateurs sont interchangeables.

## 2. Rôle de chaque paquet

| Paquet | Responsabilité | Dépendances |
|--------|----------------|-------------|
| `cmd/api` | **Composition root** : lit la config, instancie les adaptateurs, injecte les dépendances, démarre le serveur HTTP, gère l'arrêt gracieux. | tout |
| `internal/domain` | Entités (`Project`, `Service`, `Deployment`, …) + **ports** (`ProjectRepository`, `DeploymentRepository`, `ServerRepository`, `DockerEngine`) + erreurs sentinelles + constantes de labels. | *(aucune)* |
| `internal/config` | Chargement de la configuration depuis l'environnement. | — |
| `internal/logging` | Logger structuré `slog` (JSON). | — |
| `internal/store` | Implémente les 3 *Repository* via SQLite (`modernc.org/sqlite`, pur-Go). Migrations embarquées. | `domain` |
| `internal/docker` | Implémente `DockerEngine` via le SDK Docker officiel. | `domain` |
| `internal/service` | **Use-cases** : validation, résolution des overrides, tri topologique des dépendances, orchestration `up`/`down`, calcul d'état. | `domain` |
| `internal/http` | Couche de livraison **Gin** : router, middlewares (request-id, log, recovery, api-key), DTO, handlers, mapping erreur→HTTP. Sert aussi la **console web** embarquée. | `domain`, `service` |
| `internal/http/web` | **Console web** : SPA statique (vanilla JS/CSS, sans build) embarquée via `go:embed`, servie *same-origin* sur `/` (redirige vers `/app/`). Consomme uniquement `/api/v1`. | *(assets statiques)* |

## 3. Les ports (contrats)

Définis dans [`internal/domain/ports.go`](../internal/domain/ports.go).

- **`ProjectRepository`** — CRUD projets + services (avec env/ports/mounts/deps) + volumes.
- **`DeploymentRepository`** — CRUD déploiements + overrides + suivi runtime des conteneurs.
- **`ServerRepository`** — accès aux serveurs (cibles Docker Engine).
- **`DockerEngine`** — opérations moteur : `Ping`, `PullImage`, `BuildImage`,
  `CreateContainer`, `StartContainer`, `StopContainer`, `RemoveContainer`,
  `InspectContainer`, `ListContainersByLabel`, `CreateVolume`, `RemoveVolume`.

Une instance de `DockerEngine` est **liée à un seul serveur**. C'est le pivot de
l'extensibilité multi-engine (Phase 2) : il suffira d'instancier un `DockerEngine`
par serveur sans toucher à la logique métier.

## 4. Modèle conceptuel

| Concept | Définition (selon le sujet) | Implémentation |
|---------|------------------------------|----------------|
| **Image** | Image Docker (pull/build). | `service.Image` / `service.BuildContext` ; endpoints `/images/*`. |
| **Service** | Définition logique (≈ service compose). | entité `Service`. |
| **Conteneur** | Instance runtime d'un service. | entité `Container` (suivi), créé via `DockerEngine`. |
| **Volume** | Stockage persistant nommé. | entité `Volume`, créé/labellisé sur le moteur. |
| **Variable d'env.** | Paramètre du conteneur, *variable* si à fournir au déploiement. | `ServiceEnv.IsVariable` + overrides. |
| **Label** | Métadonnée. | labels de gestion `com.devnatif.*` (P3 pour labels métier). |
| **Réseau / Secret** | — | entités/handlers prévus en Phase 3. |
| **Serveur** | Instance Docker Engine (locale/distante). | entité `Server` (un seul `local` en MVP). |
| **Projet** | Description abstraite multi-conteneurs. | entité `Project` (+ services/volumes). |
| **Déploiement** | Matérialisation d'un projet sur un serveur. | entité `Deployment` (+ overrides/containers). |

## 5. Flux d'une requête : `POST /deployments/:id/up`

```
HTTP (Gin handler upDeployment)
  └─> service.DeploymentService.Up(ctx, id)
        1. Charge le déploiement (+ overrides) et les services/volumes du projet (store)
        2. Tri topologique des services selon depends_on (helpers.topoSort)
        3. Pour chaque volume : DockerEngine.CreateVolume (nom = devnatif_<dep>_<vol>, labellisé)
        4. Pour chaque service (dans l'ordre), pour chaque replica :
             - ensureImage : PullImage (image) ou BuildImage (build context)
             - resolveEnv  : env du service + overrides kind=env
             - resolvePorts: ports du service + overrides kind=port
             - resolveMounts: volumes du service -> noms moteur
             - DockerEngine.CreateContainer (+ labels com.devnatif.*) puis StartContainer
        5. SaveContainers (store) : persiste le suivi runtime
        6. computeStatus : agrège l'état réel -> running / partially-running / not-running
        7. UpdateDeployment (store)
  └─> 200 OK : déploiement à jour (statut + conteneurs)
```

## 6. Supervision pilotée par labels

Chaque ressource créée par l'API est **labellisée** :

| Label | Valeur |
|-------|--------|
| `com.devnatif.managed` | `true` |
| `com.devnatif.project` | ID du projet |
| `com.devnatif.deployment` | ID du déploiement |
| `com.devnatif.service` | ID du service |

L'état d'un déploiement est calculé **en direct** via
`ListContainersByLabel({deployment: id})` — pas de dérive entre la base et la
réalité du moteur. Idem pour `down`, qui retrouve et supprime les conteneurs par
label (robuste même si le suivi en base est incomplet).

Règle d'agrégation (`computeStatus`) :

- 0 conteneur trouvé → `not-running` (ou `pending` si jamais déployé) ;
- tous `running` → `running` ;
- sinon → `partially-running`.

## 7. Décisions techniques notables

- **SQLite pur-Go (`modernc.org/sqlite`)** : aucun CGO, donc build statique
  (`CGO_ENABLED=0`), cross-compilation triviale, image finale ~35 Mo.
- **Image multi-stage** : `golang:1.25-alpine` (build) → `alpine:3.20` (runtime).
  Go 1.25 est requis par une dépendance transitive du SDK Docker (`otelhttp`).
- **Volumes nommés par déploiement** (`devnatif_<deploymentID>_<nom>`) : isole les
  données entre déploiements d'un même projet, conformément à la séparation
  projet/déploiement.
- **Ports/env variables** (`IsVariable`) : le projet *déclare* ce qui doit être
  fourni au déploiement ; les `DeploymentOverride` fournissent les valeurs
  concrètes. C'est le mécanisme qui rend un projet réutilisable.
- **Arrêt gracieux** : `SIGINT`/`SIGTERM` → `server.Shutdown` avec timeout.

## 8. Extensibilité (P2/P3) — déjà préparée

| Évolution | Préparation présente |
|-----------|----------------------|
| **Scaling** (P2) | `Service.Replicas` + table `containers` 1..N ; `Up` boucle déjà sur les replicas (vérifié). |
| **Multi-engine** (P2) | entité `Server`, `deployments.server_id`, `DockerEngine` instanciable par serveur. |
| **Réseaux** (P3) | à ajouter : entité `Network` + table + champ réseau du conteneur. |
| **Secrets** (P3) | à ajouter : entité `Secret` + injection (montage/tmpfs/env). |
| **Labels métier** (P3) | labels déjà gérés ; exposer des labels utilisateur par service. |
| **AAA / sécurité** | middleware `apiKeyAuth` en place ; remplaçable par JWT/OAuth. |
