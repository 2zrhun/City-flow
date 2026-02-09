# City-flow — Roadmap

## Vision
Livrer une version fiable et utilisable rapidement (MVP), puis itérer vers une V1 robuste et scalable.

---

## Phase 0 — Cadrage (Semaine 1)
**Objectif :** verrouiller le périmètre fonctionnel et technique.

### Livrables
- Périmètre MVP validé
- User stories priorisées
- Critères d’acceptation définis
- Risques techniques identifiés

### KPI de sortie
- 100% des flux critiques documentés
- backlog MVP priorisé

---

## Phase 1 — Design & UX (Semaines 1–2)
**Objectif :** transformer le cadrage en parcours utilisateur clairs.

### Livrables
- Wireframes mobile/desktop
- Prototype Figma cliquable
- Revue design + itérations
- Maquette finale validée

### KPI de sortie
- 0 blocage UX sur les parcours principaux
- validation PO/équipe produit

---

## Phase 2 — Backend/API + Auth (Semaines 2–4)
**Objectif :** construire le socle applicatif.

### Livrables
- Architecture backend en place
- Base de données + migrations
- Endpoints API principaux
- Module authentification (signup/login/logout)
- Gestion rôles/permissions
- Logs + monitoring minimal
- Documentation API (OpenAPI/Postman)

### KPI de sortie
- API stable sur parcours critiques
- endpoints sécurisés (auth + rate limiting)

---

## Phase 3 — Qualité & Déploiement (Semaines 4–5)
**Objectif :** fiabiliser et préparer la mise en production.

### Livrables
- Plan de tests (unitaires + intégration)
- Exécution des tests
- Correction bugs critiques
- Déploiement staging
- Go/No-Go production + release notes

### KPI de sortie
- 0 bug bloquant (P0)
- < 3 bugs majeurs (P1)
- smoke tests staging validés

---

## Phase 4 — MVP Live (Semaine 6)
**Objectif :** mise en ligne maîtrisée.

### Livrables
- Mise en production MVP
- Monitoring actif post-release
- Correctifs rapides J+1/J+2

### KPI de sortie
- Disponibilité > 99%
- temps de réponse API parcours principal < 500ms (hors pics)

---

## Phase 5 — Post-MVP / V1 (Semaines 7+)
**Objectif :** améliorer produit et scalabilité.

### Pistes
- Analytics avancées
- Automatisations
- UX avancée / personnalisation
- Intégrations externes non critiques

---

## Risques principaux
- Glissement de périmètre MVP
- Dépendances externes (infra/auth)
- Dette technique si arbitrages trop agressifs

## Rituels de pilotage recommandés
- Daily 15 min (blocages)
- Revue roadmap hebdo (priorités)
- Démo de fin de sprint + rétro

## Définition de "Done" (globale)
Une fonctionnalité est *Done* si :
1. développement terminé,
2. tests passants,
3. revue effectuée,
4. doc minimale à jour,
5. validée métier/PO.

---

## Répartition des tâches (3 personnes)

### Rôles
- **Hamza (AdminSys / Infra)** : ownership infra, environnements, sécurité, déploiements, observabilité
- **Walid (Dev 1)** : ownership backend/API
- **Hugo (Dev 2)** : ownership frontend/UX + QA fonctionnelle

### Découpage par phase

#### Phase 0 — Cadrage
- **Hamza** : contraintes infra/sécurité, risques techniques
- **Walid** : cadrage technique API/données
- **Hugo** : cadrage parcours utilisateur + besoins UI

#### Phase 1 — Design & UX
- **Hugo (lead)** : wireframes, prototype, itérations design
- **Walid** : faisabilité technique UI/API
- **Hamza** : contraintes d’intégration (auth, performance, hébergement)

#### Phase 2 — Backend/API + Auth
- **Walid (lead)** : architecture backend, endpoints, logique métier
- **Hugo** : intégration front des APIs, validation parcours utilisateurs
- **Hamza** : DB/migrations (support), secrets, sécurité, rate limiting, logging/monitoring

#### Phase 3 — Qualité & Déploiement
- **Hamza (lead)** : CI/CD, staging, observabilité, rollback plan
- **Walid** : tests unitaires/intégration backend + corrections
- **Hugo** : tests E2E/fonctionnels + retours UX

#### Phase 4 — MVP Live
- **Hamza (lead)** : release prod, supervision, incidents
- **Walid** : correctifs backend prioritaires
- **Hugo** : correctifs UI/UX prioritaires + feedback utilisateur

### Charge cible (approx.)
- **Hamza (Infra/AdminSys)** : 30%
- **Walid (Dev 1)** : 35%
- **Hugo (Dev 2)** : 35%

### Règles d’assignation Trello
- Toute carte doit avoir **1 owner principal** + **1 reviewer**
- Cartes Infra/Sécurité/Deploy → owner **Hamza**
- Cartes API/Backend → owner **Walid**
- Cartes UI/UX/Intégration front → owner **Hugo**
- Review croisée recommandée :
  - Walid ↔ Hugo sur features
  - Hamza ↔ Walid sur sécurité/perf

---

## Phase 0 - Dossier de cadrage actionnable

### Validation du perimetre MVP final (inclus / exclu)

#### Inclus dans le MVP
- Ingestion IoT via MQTT (`cityflow/traffic/{sensor_id}`)
- Stockage time-series dans TimescaleDB (`traffic_raw`)
- Service `predictor` avec baseline explicable (T+30)
- Service `rerouter` par regles (ETA/CO2 indicatif)
- API minimale (`/health`, `/api/traffic/live`, `/api/predictions`, `/api/reroutes/recommended`, `WS /ws/live`)
- Dashboard de demo live (carte + graphes + recommandations)
- Deploiement local conteneurise + runbook demo
- Observabilite minimale (Prometheus + Grafana + health endpoints)

#### Exclu du MVP (post-MVP / bonus)
- Modele ML avance (LSTM/XGBoost)
- Istio/service mesh
- Multi-tenant, SSO entreprise, IAM avance
- Haute dispo multi-zone / PRA complet
- ArgoCD complet (bonus si temps restant)
- Enrichissement meteo/evenements en production

### User stories MVP priorisees

| ID | Priorite | User story | Owner | Reviewer |
|---|---|---|---|---|
| US-01 | P0 | En tant qu'operateur, je vois un flux trafic live ingere depuis MQTT vers la base. | Walid | Hamza |
| US-02 | P0 | En tant qu'operateur, je consulte les mesures recentes via API pour alimenter le dashboard. | Walid | Hugo |
| US-03 | P0 | En tant qu'operateur, je recois une prediction de congestion T+30 par axe. | Walid | Hugo |
| US-04 | P0 | En tant qu'operateur, je vois des recommandations de reroutage en cas de congestion. | Walid | Hugo |
| US-05 | P0 | En tant qu'admin infra, je deploie et relance la stack localement en une commande. | Hamza | Walid |
| US-06 | P0 | En tant qu'admin infra, je supervise l'etat des services et les metriques critiques. | Hamza | Walid |
| US-07 | P1 | En tant qu'operateur, je visualise sur dashboard la carte + graphes live. | Hugo | Walid |
| US-08 | P1 | En tant qu'admin infra, je securise les credentials et l'acces broker en environnement cible. | Hamza | Walid |

### Criteres d'acceptation (par user story)

- `US-01`
  - Given le simulateur actif, when 1 minute s'ecoule, then des lignes sont presentes dans `traffic_raw`.
  - Le collector rejette les messages invalides et incremente un compteur d'erreur.
- `US-02`
  - `GET /api/traffic/live` repond en < 500 ms sur jeu de charge nominal.
  - Le payload contient au minimum `ts`, `road_id`, `speed_kmh`, `flow_rate`, `occupancy`.
- `US-03`
  - Une prediction `horizon=30` est produite automatiquement au plus tard toutes les 5 minutes.
  - Les valeurs sont persistantes en base et exploitables par API/dashboard.
- `US-04`
  - Si congestion > seuil defini, au moins une recommandation est generee avec motif.
  - Les recommandations sont exposables via endpoint dedie.
- `US-05`
  - `docker compose up -d --build` demarre les composants MVP sans erreur bloquante.
  - Un runbook permet a un membre equipe de reproduire la demo en moins de 15 minutes.
- `US-06`
  - Chaque service expose `health` et/ou `metrics`.
  - Prometheus scrape au minimum collector + prometheus; Grafana dispose d'une datasource preconfiguree.
- `US-07`
  - Le dashboard affiche carte + graphes live sans blocage UX majeur sur parcours principal.
  - Les donnees live se rafraichissent automatiquement (polling/WS).
- `US-08`
  - Aucun secret reel n'est committe en clair.
  - En cible, MQTT n'autorise pas `allow_anonymous true`; un mecanisme d'authentification est actif.

### Contraintes infra / securite (Phase 0)

#### Secrets et mots de passe
- Interdiction de credentials de production dans Git.
- Fichier `.env.example` autorise uniquement des valeurs de dev.
- Rotation des secrets avant passage staging/prod.
- Convention: secrets injectes par variables d'environnement ou secret manager (pas en dur dans le code).

#### Auth MQTT (cible)
- Local dev: `allow_anonymous true` tolere pour acceleration.
- Staging/prod: `allow_anonymous false` obligatoire.
- Auth minimale cible: user/password broker + ACL par topic (`cityflow/traffic/+` en publish, topics strictement necessaires en subscribe).

#### Observabilite minimale
- Endpoint `health` pour chaque service critique.
- Endpoint `/metrics` Prometheus pour collector/predictor/rerouter/api.
- Dashboard Grafana minimum:
  - Disponibilite services
  - Debit ingestion (messages/s)
  - Taux d'erreur ingestion
  - Latence API p95

#### Contraintes perf / hebergement (MVP)
- Cible latence ingestion MQTT -> DB: p95 < 2 s.
- Cible API parcours principal: p95 < 500 ms (hors pics).
- Capacite nominale MVP: 10-50 capteurs, intervalle 1-2 s.
- Hebergement MVP: machine locale/dev ou VM unique (pas de HA requise).

### Backlog MVP priorise (owner + reviewer)

| Priorite | Carte | Description | Owner | Reviewer | Definition of done |
|---|---|---|---|---|---|
| P0 | BL-01 Setup stack locale | Compose up + health checks services socle | Hamza | Walid | Tous les conteneurs `healthy` + runbook valide |
| P0 | BL-02 Ingestion MQTT -> DB | Simulateur publie, collector persiste `traffic_raw` | Walid | Hamza | Donnees visibles en DB + metriques collector |
| P0 | BL-03 Schema DB MVP | Tables `traffic_raw`, `predictions`, `reroutes` creees | Walid | Hamza | Migrations executees sans erreur |
| P0 | BL-04 Predictor T+30 baseline | Job periodique de prediction persistante | Walid | Hugo | Endpoint/API exploitable + tests mini |
| P0 | BL-05 Rerouter heuristique | Generation alternatives selon seuil congestion | Walid | Hugo | Resultats consultables et coherents |
| P0 | BL-06 API minimale + WS | Exposition endpoints MVP + flux live | Walid | Hugo | Contrat respecte + temps reponse cible |
| P0 | BL-07 Observabilite MVP | Prometheus scrape + Grafana datasource/dashboard | Hamza | Walid | KPI techniques visibles |
| P0 | BL-08 Hygiene securite MVP | Gestion secrets dev + spec auth MQTT cible | Hamza | Walid | Checklist securite Phase 0 validee |
| P1 | BL-09 Dashboard demo | Carte + graphes + panneau reroutage | Hugo | Walid | Demo bout en bout validee |
| P1 | BL-10 Packaging demo | Scripts de demo + README finalise | Hamza | Hugo | Soutenance reproductible |

### Matrice des risques techniques

| ID | Risque | Probabilite | Impact | Mitigation | Owner |
|---|---|---|---|---|---|
| R-01 | Glissement de scope MVP | Elevee | Eleve | Geler P0, repousser bonus en P1/P2, revue hebdo scope | Hamza |
| R-02 | Instabilite broker MQTT | Moyenne | Eleve | Healthcheck, retry client, tests de charge legers | Hamza |
| R-03 | Saturation DB/latence ecriture | Moyenne | Eleve | Index, hypertable, batch/retention si besoin | Walid |
| R-04 | Qualite donnees capteurs insuffisante | Moyenne | Moyen | Validation schema payload + rejet + metrique erreur | Walid |
| R-05 | Prediction non pertinente pour demo | Moyenne | Moyen | Baseline explicable + seuils calibrables + limites assumees | Walid |
| R-06 | Faille securite (secrets, MQTT anonyme) | Moyenne | Eleve | Regles secrets + auth broker en cible + revue config | Hamza |
| R-07 | Manque de visibilite incidents | Moyenne | Eleve | Metrics/health obligatoires + dashboard ops minimum | Hamza |
| R-08 | Dette technique acceleree | Elevee | Moyen | DoD stricte: tests mini + review croisee + doc minimale | Hugo |

### Checklist de sortie Phase 0
- [ ] Perimetre MVP final valide (in/out)
- [ ] User stories MVP priorisees et assignees
- [ ] Criteres d'acceptation valides pour chaque US
- [ ] Contraintes infra/securite validees (secrets, MQTT cible, observabilite, perf)
- [ ] Backlog MVP priorise avec owner + reviewer
- [ ] Matrice risques (probabilite/impact/mitigation/owner) validee en equipe
