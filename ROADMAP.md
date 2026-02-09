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
