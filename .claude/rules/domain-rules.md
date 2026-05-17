---
paths:
  - "src/**"
---

# Domain rules

`src/` is DevPulse's pure domain layer, fully decoupled from the Laravel framework. Every business concept (VO, Entity, Port interface, Domain Service) lives here; framework adapters (Eloquent Model, HTTP Controller, Filament Resource, Saloon Connector) belong in `app/`.

Read this document before touching anything under `src/`. Any PR that breaks these rules must pay down the debt before it can be merged.

---

## 1. Directory layout

```
src/
├── DevPulse/        # business namespace (composer psr-4: DevPulse\)
│   ├── Ci/          # CI domain (Build, BuildStatus, BuildTrigger, CiProvider port…)
│   ├── Shared/      # cross-domain VOs (CommitSha, MonthRange, RepoId, RepoFullName…)
│   └── Vcs/         # VCS domain (PullRequest, Author, Platform, Factory…)
└── Tests/           # matching domain tests (composer psr-4: Tests\Domain\)
    ├── Ci/
    ├── Shared/
    └── Vcs/
```

| Directory | Namespace | Purpose |
| --- | --- | --- |
| `src/DevPulse/` | `DevPulse\…` | Domain code |
| `src/Tests/` | `Tests\Domain\…` | Tests for domain code; **mirrors** the subdirectory layout of `src/DevPulse/` |

**Domain tests live alongside domain code under `src/`**: the test for `src/DevPulse/Vcs/PullRequest.php` is at `src/Tests/Vcs/PullRequestTest.php` — don't go digging in `tests/`.
`tests/` is reserved for framework integration tests (Feature/Unit/Aggregation/Persistence/Console/Filament/Infrastructure — the Laravel-aware layers).

---

## 2. Forbidden namespaces (hard rule)

**Any file under `src/` may only depend on:**

1. Other namespaces inside `src/` (`DevPulse\…`)
2. PHP native and SPL types, e.g. `DateTimeImmutable`, `InvalidArgumentException`, `Stringable`, `Generator`
3. `PHPUnit\Framework\TestCase` (only under `src/Tests/`); architecture tests may additionally use `PHPat\…`

**Why**: the domain must be testable without booting Laravel — and without any third-party composer package installed.

PHP native and SPL types are part of the language platform, not external dependencies — if PHP runs, they're there.

### When you need an external capability: use Dependency Inversion (DIP)

If the domain genuinely needs time, HTTP, randomness, ID generation, etc., **don't** import the external library directly. Instead:

1. Define a port interface in `src/` (the domain owns the contract)
2. Write the adapter in `app/`, implementing that interface
3. Have the caller (framework layer) inject the adapter at composition time

Port interfaces live under `src/` and follow the same dependency rules as every other file in `src/`.
