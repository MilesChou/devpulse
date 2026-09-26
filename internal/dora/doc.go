// Package dora computes the four DORA delivery-performance metrics from
// already-persisted facts. It is pure: no DB, no clock, no I/O — callers
// load a window's Input and Compute turns it into a Report.
//
// Definitions (see openspec/changes/add-dora-metrics/design.md, D6):
//
//   - Deployment: a PR merged into the repo's default branch, at merged_at.
//   - Lead Time for Changes: earliest commit author time → merged_at.
//   - Change Failure Rate: remediation deployments (revert or hotfix) ÷
//     deployments.
//   - Failed Deployment Recovery Time: reverted merge → revert merge, and
//     incident opened → resolved.
//
// Every event is attributed to the window by its end time.
package dora
