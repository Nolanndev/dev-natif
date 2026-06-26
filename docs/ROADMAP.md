# Roadmap & couverture du sujet

## Couverture du cadre conceptuel

| Concept du sujet | Phase requise | État |
|------------------|---------------|------|
| Image (pull/build) | P1 | ✅ Implémenté |
| Service | P1 | ✅ |
| Conteneur | P1 | ✅ (création + suivi) |
| Volume | P1 | ✅ |
| Variable d'environnement | P1 | ✅ (+ overrides) |
| Serveur | P1 (local) | ✅ (un `local` ; abstraction prête P2) |
| Projet | P1 | ✅ |
| Déploiement | P1 | ✅ |
| État (running/partial/not-running) | P1 | ✅ |
| Dépendances entre services | (bonus P1) | ✅ (tri topologique à l'`up`) |
| Scaling (replicas) | P2 | ✅ **fonctionnel** (alias DNS partagé) |
| Multi-engine | P2 | 🟡 Préparé (entité `Server`, engine par serveur) |
| Réseau (Network) | P3 | 🟡 **Réseau par défaut par déploiement** (linking DNS) fait ; réseaux nommés multiples à exposer |
| Secret | P3 | ⬜ Crochet prévu |
| Label métier | P3 | 🟡 Labels de gestion en place ; labels utilisateur à exposer |
| Sécurité (AAA) | transverse | ✅ **Tokens JWT** (login/refresh/expiration) |
| Journalisation | transverse | ✅ slog JSON + request-id + log d'accès |
| Historique / événements | transverse | ✅ Table `events` + erreurs daemon persistées |
| Logs des conteneurs | transverse | ✅ Via API/UI (lecture en direct) |
| Rétention des données | transverse | ✅ Purge auto > 30 j (configurable) |

## Phase 1 — MVP / POC ✅

Périmètre exigé par le sujet, **livré et vérifié** contre le Docker Engine local :
interaction moteur local, description de projet, déploiement, état d'un déploiement,
gestion services/conteneurs, images (pull/build), volumes, variables d'environnement.

## Phase 2 — Multi-engine + scaling

- **Scaling** : `Service.Replicas`, la boucle `Up` crée N conteneurs, l'état gère
  déjà `partially-running`. **Opérationnel.**
- **Multi-engine** — travail restant :
  1. CRUD `servers` (POST/PUT/DELETE) + test de connectivité (`Ping`).
  2. Une `factory` de `DockerEngine` indexée par `server_id` (cache d'instances).
  3. `DeploymentService` sélectionne le moteur via `deployment.server_id`.
  4. Pas de changement de schéma (déjà `deployments.server_id`).

## Phase 3 — Réseaux, secrets, labels

- **Réseaux** : entité `Network` + table + `service_networks` ; création réseau à
  l'`up`, rattachement des conteneurs ; endpoints `/projects/:id/networks`.
- **Secrets** : entité `Secret` + injection (montage fichier/tmpfs ou env) ;
  attention à ne jamais journaliser les valeurs.
- **Labels métier** : exposer des labels utilisateur par service (ex. règles
  Traefik), fusionnés avec les labels `com.devnatif.*`.

## Dette / améliorations possibles

- Tests d'intégration automatisés (le smoke-test manuel est documenté dans le README).
- Migrations versionnées (table de versions) plutôt qu'un script idempotent unique.
- Streaming des logs de `pull`/`build` vers le client (SSE/WebSocket).
- Validation plus riche des entrées (schémas, bornes de ports, etc.).
- Pagination des listes.
