# CLAUDE.md

## Status

This system is **pre-release**. Backward compatibility is not a constraint —
anything can be rewritten or redone, including database migrations, schema,
CLI surface, and on-disk formats. Prefer the right design over a compatible
one.

## Language

- Use English in PRs and all code internals (comments, variable names, commit messages, etc.).

## Documentation

- `README.md` (English) and `README.zh-TW.md` (Traditional Chinese, Taiwan) must stay in sync. Whenever you change one, mirror the change in the other in the same commit.

## Project goal

DevPulse exists to **observe CI and PR engineering-efficiency metrics**.
The list below enumerates the data dimensions we plan to surface.
Treat this list as the priority driver for design and implementation: every domain model, port, and adapter must be able to support computing these metrics.

### CI metrics to observe

- **Average build failure rate**
  - Ideal: 0
- **Average re-push count per PR**
  - Ideal: 1
- **Average PR merged − created duration**
  - Ideal: 24h
- **PR size distribution**
  - Ideal: skewed toward small PRs

### Implications for domain design

- PR timestamps (created / ready / merged / closed) must be **precise**, not approximated — they are the denominator of lead-time metrics.
- Build outcomes must be traceable back to PRs; re-push count must be derivable from the commit/build sequence.
- ChangeStats (additions / deletions) must be retained — it is the basis for PR size distribution.
