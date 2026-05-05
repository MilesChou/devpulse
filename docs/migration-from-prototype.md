# 從 Python prototype 遷移到 devpulse

> **目標讀者**：未來的我（或接手的人）。
>
> **核心問題**：什麼時候可以停止維護 Python prototype（`ci_analysis`）？

## 背景

devpulse 是 `ci_analysis`（Python prototype）的「正規版本」，繼承相同的 domain knowledge，但用 PHP Laravel 重寫，並引入 PostgreSQL 持久化層。

兩者目前**並存**：

| | Python prototype | devpulse |
|---|---|---|
| **角色** | reference / golden output | 長期維護版本 |
| **資料儲存** | file-based cache | PostgreSQL |
| **設定** | `.profile.yaml`（綁定特定組織） | DB（CLI 維護，可重複部署於不同團隊） |
| **輸出** | markdown report | markdown + Stage 2 接 Grafana |
| **狀態** | 凍結，僅維持可執行 | 持續開發 |

並存的理由（[spec design.md Decision 6](../openspec/changes/propose-devpulse/design.md#decision-6python-prototype-保留作為-reference--golden-output)）：

1. **Joel Spolsky 的「不要重寫」鐵則的緩衝**：保留可運作 reference，重寫過程能對照
2. **golden output**：devpulse 第一版的驗證標準是「同月份、同設定下，產出與 Python 一致的數值」
3. **bug 對照**：重寫過程中發現邏輯不一致，可立刻判斷哪邊對

## 退場條件（Sunset Criteria）

當以下**所有**條件達成，Python prototype 可以正式停止維護：

### 必要條件

- [ ] **數值一致性已驗證**：spec 第 9 章（task 9.1~9.4）完成，至少 3 個月份的 markdown 報告與 Python 版數值完全一致（失敗率、PR review latency、build 計數三項核心指標）
- [ ] **devpulse 已能獨立完成日常觀測流程**：fetch + aggregate + report 整條 pipeline 跑得起來，不需要回頭跑 Python 版
- [ ] **已連續使用 devpulse 至少 3 個月**：實際使用過、發現過 bug、修過、再驗證——確保不是 ceremony 跑完就棄用
- [ ] **Stage 2 Grafana dashboard 已上線**或 explicitly 不打算上：表示 devpulse 的 schema 設計已經被「真實 query」驗證過

### 可選條件（加分但非必要）

- [ ] devpulse 已經被部署到第二個團隊驗證（證明「可重複部署於不同團隊」這個 Goal 達成）
- [ ] human signals classifier 在 devpulse 端達到與 Python 等價或更好的分類準確度

## 退場執行步驟

當條件達成、決定 retire Python prototype 時：

1. **在 prototype repo 加一份 `DEPRECATED.md`**，指向本 repo
2. **凍結 Python repo 的 default branch**：archive GitHub repo（保留 read-only）
3. **更新本 README**：把「Python prototype 為 reference / golden output」改成「歷史背景：本工具源自已封存的 ci_analysis prototype」
4. **更新 [design.md Decision 6](../openspec/changes/propose-devpulse/design.md#decision-6python-prototype-保留作為-reference--golden-output)**：標註為 "superseded"，附上 retire 日期與當時的 commit hash

## 不會做的事

- **不會刪除 Python prototype repo**：歷史檔案保留，純粹凍結維護
- **不會自動轉換 Python `.cache/` 到 devpulse DB**：[design.md Open Questions](../openspec/changes/propose-devpulse/design.md#open-questions) 已決定不做這個工具
- **不會把 `.profile.yaml` 自動匯入 devpulse 的 DB**：devpulse 的設定一律走 Artisan command 重新建立（避免 schema 綁住、且 DB 已成為 source of truth）

## 為什麼不直接砍掉 Python prototype？

短期 retire 的風險：

- 失去 golden output → 重寫過程中發現算錯了沒得對照
- 萬一 devpulse 有 critical bug 沒有 fallback
- 心智成本：停掉 Python 時若 devpulse 還沒完全長好，反而更焦慮

只有當「devpulse 跑了一段時間、證明能完整取代」之後才 retire，這才是 Joel Spolsky 鐵則的正確應用——不是「不要重寫」，而是「重寫期間保留 reference」。

## 進度追蹤

對照 [`openspec/changes/propose-devpulse/tasks.md`](../openspec/changes/propose-devpulse/tasks.md) 中第 9 章「Golden output 對照」進度，是判斷 retire 時機最直接的依據。

當第 9 章全部完成 + 上述「必要條件」全部 ✅，就可以開始走「退場執行步驟」。
