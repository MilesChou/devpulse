---
name: devpulse-setup
description: >
  初始化 DevPulse 觀測群體，包含建立 group、新增 member、新增 repo，透過 Laravel artisan 指令批次執行。
  當使用者提供截圖、表格或資料，要新增 group、member、repo 時，一律使用此 skill。
  觸發情境：「新增 group」、「新增 member」、「新增 repo」、「建立觀測群體」、「初始化 DevPulse」、「加入成員」、「加入 repo」、提供含有 slug/github_account/full_name 欄位的截圖或資料。
---

# DevPulse Setup Skill

從使用者提供的截圖或表格資料中擷取資訊，組合對應的 artisan 指令並批次執行。

## 三個操作指令

### 1. 建立 Group

```bash
php artisan devpulse:group:create {slug} --description="{description}"
```

| 參數 | 說明 | 規則 |
|------|------|------|
| `slug` | 群體識別碼（必填） | 小寫英數與 dash，例如 `ac`、`frontend-team` |
| `--description` | 描述（選填） | 任意文字 |

### 2. 新增 Member

```bash
php artisan devpulse:member:add {group} {github_account} {display_name}
```

| 參數 | 說明 |
|------|------|
| `group` | group slug（必須已存在） |
| `github_account` | GitHub 帳號，區分大小寫 |
| `display_name` | 顯示名稱 |

### 3. 新增 Repo

```bash
php artisan devpulse:repo:add {group} {full_name}
```

| 參數 | 說明 | 格式 |
|------|------|------|
| `group` | group slug（必須已存在） | — |
| `full_name` | 完整 repo 名稱 | `owner/name`，例如 `104corp/ac-api-php` |

## 執行流程

使用者可能一次提供一種資料（只有 group、只有 members、只有 repos），也可能三種都給。

1. **判斷資料類型**：從截圖欄位名稱辨識是哪種資料
   - 含 `slug` → 建立 group
   - 含 `github_account` / `display_name` → 新增 member
   - 含 `full_name` → 新增 repo

2. **詢問 group slug**（若不明確）：新增 member 和 repo 都需要指定 group，若使用者未說明，先確認。

3. **組合指令批次執行**：用 `&&` 串接，逐一顯示執行結果。

4. **確認結果**：列出成功新增的項目摘要。

## 注意事項

- Group 必須先存在，才能新增 member 或 repo；若 group 不存在會報錯，這時先建立 group 再重試。
- Member 跨 group 共用，同一個 `github_account` 加入不同 group 不會重複建立。
- `display_name` 含有空格時，指令中需加引號：`"Display Name"`。

## 範例

### 建立 group
使用者輸入：截圖顯示 `slug=ac`、`description=測試用：104corp 兩個 repo`

```bash
php artisan devpulse:group:create ac --description="測試用：104corp 兩個 repo"
```

### 新增 members（批次）
使用者輸入：截圖顯示 6 位成員，group 為 `ac`

```bash
php artisan devpulse:member:add ac MilesChou Miles && \
php artisan devpulse:member:add ac XiuHanYang Hannah && \
php artisan devpulse:member:add ac ChengKaiChiang Ray && \
php artisan devpulse:member:add ac pruelinadi09 Prue && \
php artisan devpulse:member:add ac LynnHung1206 Lynn && \
php artisan devpulse:member:add ac TJChiang Nature
```

### 新增 repos（批次）
使用者輸入：截圖顯示 5 個 repo，group 為 `ac`

```bash
php artisan devpulse:repo:add ac 104corp/ac-api-php && \
php artisan devpulse:repo:add ac 104corp/signin.104.com.tw && \
php artisan devpulse:repo:add ac 104corp/bsignin.104.com.tw && \
php artisan devpulse:repo:add ac 104corp/accounts.104.com.tw && \
php artisan devpulse:repo:add ac 104corp/ac-manager.104.com.tw
```
