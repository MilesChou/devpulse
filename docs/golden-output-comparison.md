# Golden Output 對照手冊

> **目標**：在 devpulse 取代 Python prototype（`ci_analysis`）的過渡期，建立可重複、可驗證的「兩版本數值對得起來」的流程。

## 為什麼要做這件事

依照 [design.md Decision 6](../openspec/changes/propose-devpulse/design.md#decision-6python-prototype-保留作為-reference--golden-output)：「devpulse 第一版的驗證標準是『同月份、同設定下，產出與 Python 一致的 markdown 報告』」。

這個對照不是 nice-to-have，而是 **retire Python prototype 的必要條件**（詳見 [docs/migration-from-prototype.md](migration-from-prototype.md)）。

## 必須一致的核心數值

以下數值在「同月份、同 group、同 bot 排除清單」前提下，**MUST 完全相等**：

### 第一級（必對）—— 基礎計數，不容差

| 指標 | devpulse 來源 | Python 來源 |
|---|---|---|
| 該月 build 總筆數（依 trigger 拆） | `BuildFailureRateQuery` 的 total 欄相加 | prototype 的 build count |
| 該月 PR 總筆數（含 / 不含 draft） | `PullRequest::query()->count()` | prototype 的 PR count |
| 失敗 build 清單長度 | `FailedBuildListQuery::run()->count()` | prototype 的 failed list |

> **不容差**：差 1 筆都要查清楚原因。最常見的原因是時區邊界（月初 / 月末 build 被劃到不同月）。

### 第二級（必對）—— 比率與聚合，容許 1e-9 浮點誤差

| 指標 | devpulse 來源 | Python 來源 |
|---|---|---|
| 個人失敗率（每位成員 × repo） | `BuildFailureRateQuery` 的 `rate` | prototype `failure_rate by author` |
| PR review latency 中位數（依 size bucket） | `ReviewLatencyQuery` 的 `latencyHours` 中位數 | prototype `latency by size` |
| daily build duration 中位數 | `DailyBuildDurationQuery` 的 `medianSeconds` | prototype `daily duration` |

> **容差**：浮點數比較使用 `abs(a - b) < 1e-9`，避免 IEEE 754 表示誤差。

### 第三級（建議對）—— 視覺化呈現，可有合理差異

- markdown 表格的欄位順序、欄寬可不同
- 失敗 build 清單的顯示時間格式可不同（例如 `Y-m-d H:i` vs `Y-m-d H:i:s`）
- ASCII bar chart 的字元寬度可不同

> **不必對**：這些是 view layer 的細節，只要數字本身對得上就好。

## 對照執行流程

### 前置條件

- [ ] devpulse 已能跑 fetch + report 完整 pipeline（spec 後續加上 fetch command 後才能做）
- [ ] Python prototype 仍可執行
- [ ] 一個有實際資料的月份（建議至少 50 筆 build / 10 個 PR 才有統計意義）
- [ ] 兩邊的設定（bot 排除清單、PR size buckets）已對齊

### Step 1：Python prototype 跑出 golden

```bash
cd /path/to/ci_analysis
python -m ci_analysis report --month 2026-04 --output golden-2026-04.md
```

產物：`golden-2026-04.md`

### Step 2：devpulse 跑同月份

```bash
cd /path/to/devpulse
# 先確保已 fetch 該月資料（fetch command 完成後）
# php artisan devpulse:fetch <group> 2026-04
php artisan devpulse:report 2026-04 --group=<group> --output=devpulse-2026-04.md
```

產物：`devpulse-2026-04.md`

### Step 3：核心數值比對

對照「必須一致的核心數值」章節，逐項檢查。建議用 `php artisan tinker` 直接 query DB 取數值，避免 markdown 渲染差異干擾比對：

```php
use App\Aggregation\BuildFailureRateQuery;
use App\Aggregation\Filter\BuildEventFilter;
use App\Domain\Shared\MonthRange;
use App\Models\Group;

$group = Group::where('slug', 'team-a')->first();
$results = (new BuildFailureRateQuery(new BuildEventFilter()))
    ->run($group, MonthRange::fromString('2026-04'));

$results->map(fn ($r) => [
    'repo' => (string)$r->repoFullName,
    'author' => $r->authorAccount,
    'total' => $r->total,
    'failures' => $r->failures,
    'rate' => $r->rate,
])->dump();
```

### Step 4：發現差異時

依照差異類型對應的處理：

| 差異類型 | 可能原因 | 處理 |
|---|---|---|
| build 計數差 1~2 筆 | 時區邊界（UTC vs 本地時間）、月底跨日 build | 確認雙方都用 UTC + 半開區間 `[start, end)` |
| 某成員失敗率差很大 | bot 過濾清單不一致、author email 對應問題 | 對齊 `excluded_bots` config |
| PR review latency 缺少某些 PR | draft handling 不同 | 確認雙方對 `ready_at IS NULL` 的處理一致 |
| daily duration 中位數差 | 中位數算法不同（偶數筆時的取法） | devpulse 用「兩中間數平均」（[`DailyBuildDurationQuery::calcMedian()`](../app/Aggregation/DailyBuildDurationQuery.php)） |
| 某筆 PR review latency 多 / 少幾小時 | first_review_at 解析時區不同 | 確認 GitHub API 回傳的 ISO 8601 解析行為一致 |

> 詳見 [design.md Risk 段](../openspec/changes/propose-devpulse/design.md#risks--trade-offs)：「PHP/Carbon 對 ISO 8601 'Z' 後綴的解析行為跟 Python `fromisoformat` 不同」。

### Step 5：紀錄差異原因

每次發現差異要做：

1. 在本 doc 末尾的「**已知差異紀錄**」段落新增一條
2. 修正 PHP 版（除非確認是 Python 版的 bug）
3. 加 unit test 防止回歸

## 已知差異紀錄

> 尚無紀錄。發現差異後請追加。

格式範例：

```
### 2026-MM-DD：差異描述

- **發現月份**：2026-04
- **發現指標**：alice × your-org/repo-a 失敗率
- **Python 值**：0.333（5/15）
- **devpulse 值**：0.357（5/14）
- **差 1 筆 build 原因**：Python 版把 author=null 的 build 算進總數，devpulse 已過濾
- **判定**：devpulse 行為正確（spec 4.6 明確要求過濾 bot / null author）
- **追加 fixture test**：`tests/Feature/Aggregation/BuildFailureRateQueryTest.php::testExcludesBuildsWithNullAuthor`
```

## 退出條件

當以下達成可正式宣告「devpulse 數值與 Python 等價」：

- [ ] 至少 3 個不同月份完成對照
- [ ] 第一級（基礎計數）100% 一致
- [ ] 第二級（比率聚合）所有差異都已紀錄並判定為「設計差異」或「devpulse 修正後一致」
- [ ] 對照流程已自動化（理想）或手動 checklist 化（最低）

達成後可開始走 [docs/migration-from-prototype.md](migration-from-prototype.md) 的退場流程。

## 相關規範

- [design.md Decision 6](../openspec/changes/propose-devpulse/design.md#decision-6python-prototype-保留作為-reference--golden-output) — 為什麼保留 Python prototype
- [openspec tasks 9.1~9.4](../openspec/changes/propose-devpulse/tasks.md) — 第 9 章 task 列表
