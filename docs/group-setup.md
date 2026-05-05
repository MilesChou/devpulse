# Group / Member / Repo 設定指南

devpulse 用 **group** 把「要一起觀察的 repo + 成員」綁在一起。所有指標（失敗率、review latency 等）都是「group × month」這個切片下的數字，所以第一步永遠是設定 group。

## 概念模型

```
Group ─┬─ has many ─→ Repo  (透過 group_repos)
       └─ has many ─→ Member (透過 group_members)
```

- **Group**：觀測群體。例如「team-platform」、「team-mobile」
- **Repo**：被觀測的 GitHub repository，用 `owner/name` 唯一識別
- **Member**：被觀測的成員，用 GitHub account 唯一識別
- **多對多**：同一個 repo / member 可以同時屬於多個 group（例如 cross-team）

## 設定流程

### Step 1：建立 group

```bash
php artisan devpulse:group:create <slug> [--description="..."]
```

- `slug`：識別字，**只允許小寫英數與 dash**（例如 `my-team`、`team-platform`）
- `--description`：人類可讀的描述（選填）

範例：

```bash
php artisan devpulse:group:create team-platform --description="Platform Team"
```

### Step 2：加 repo 到 group

```bash
php artisan devpulse:repo:add <group-slug> <owner/name>
```

- 如果 repo 不存在會自動建立
- 同一 repo 重複加進同一 group 會被識別為冪等操作（warn 但 exit 0）

範例：

```bash
php artisan devpulse:repo:add team-platform your-org/api-server
php artisan devpulse:repo:add team-platform your-org/web-frontend
```

### Step 3：加成員到 group

```bash
php artisan devpulse:member:add <group-slug> <github-account> <display-name>
```

- `github-account`：成員的 GitHub login（不含 `@`）
- `display-name`：報告中顯示的名字
- 如果成員不存在會自動建立

範例：

```bash
php artisan devpulse:member:add team-platform alice "Alice Chen"
php artisan devpulse:member:add team-platform bob "Bob Lin"
```

### Step 4：驗證

```bash
php artisan tinker
```

```php
App\Models\Group::with('repos', 'members')->get()->toArray();
```

## 多 group 切換

devpulse 設計支援多 group 並存。實際使用情境：

- 一人觀測多個團隊（例如 manager / 顧問角色）
- 同 repo 跨 team（例如平台 repo 同時被 platform team 和 product team 觀察）
- 過渡期：建一個臨時 group 對照新舊組織分組

切換只是換 `--group` 參數：

```bash
php artisan devpulse:report 2026-04 --group=team-platform
php artisan devpulse:report 2026-04 --group=team-mobile
```

> ⚠️ `devpulse:report` 尚未實作（spec 第 8 章），上面是目標形態。

## Bot 過濾

devpulse 預設排除常見 bot 開的 PR / 留的 review，避免污染統計。預設清單在 `config/devpulse.php`：

```php
'excluded_bots' => [
    'dependabot[bot]',
    'dependabot-preview[bot]',
    'renovate[bot]',
    'github-actions[bot]',
    'copilot-pull-request-reviewer[bot]',
],
```

要新增 bot 直接編輯 config 即可（這是「跨 group 共用、不變」的設定，不放 DB）。

## PR Size 分桶

PR 改動行數（additions + deletions）的分桶規則也在 `config/devpulse.php`：

```php
'pr_size_buckets' => [
    'XS' => 50,    // < 50 行
    'S' => 200,    // 50 ~ 199
    'M' => 500,    // 200 ~ 499
    'L' => 1000,   // 500 ~ 999
    'XL' => null,  // ≥ 1000
],
```

值是「上限（不含）」，最後一桶寫 `null` 代表無上限。要客製化分桶閾值就改這份 config。

## Human Signals（規劃中）

> ⚠️ **尚未實作**（spec 第 7 章 task 7.1~7.3）

未來規劃：每個 repo 可以設定一組 `human_signals`，讓 classifier 把失敗 build 的 log 做字串比對，分類為「人為錯誤（lint、test）」vs「環境問題（infra、flake）」。

設計形態（暫定，以實作為準）：

```
human_signals (JSON column on repos table):
  - { "category": "lint", "pattern": "PHPCS:" }
  - { "category": "test", "pattern": "Tests failed:" }
  - { "category": "type", "pattern": "phpstan: Found" }
```

實作完成後本段會更新具體的 CLI 設定方式。

## 常見操作

### 列出所有 group

```bash
php artisan tinker --execute="App\Models\Group::all(['slug', 'description'])"
```

### 移除成員 / repo（暫無 CLI）

目前要透過 tinker 處理：

```bash
php artisan tinker
```

```php
$group = App\Models\Group::where('slug', 'team-platform')->first();
$group->members()->detach($memberId);
$group->repos()->detach($repoId);
```

### 刪除整個 group

```php
App\Models\Group::where('slug', 'old-team')->delete();
// pivot table 紀錄會被 cascade 刪除（但 Member / Repo 本身保留）
```
