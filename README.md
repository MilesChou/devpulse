# devpulse

研發效能觀測工具：從 GitHub / CI 抓資料、聚合成團隊指標（CI 失敗率、PR review latency、build duration、PR 重跑次數），最終以 markdown 月報或 Grafana dashboard 呈現。

從 Python prototype（`ci_analysis`）改寫的「正規版本」，目標是長期維護、可重複部署於不同團隊。

## 定位

- **是什麼**：本機自用的 CLI 工具 + PostgreSQL 資料層
- **不是什麼**：SaaS、多租戶、即時 webhook 服務、Web Dashboard（dashboard 規劃由 Stage 2 改用 Grafana 直連 DB）
- **量級**：單機、單一使用者、單月單 repo 約 100~1000 筆 build 的小資料量

## 安裝

### 環境需求

- PHP **8.3+**
- Composer
- PostgreSQL（開發階段也可先用 SQLite）
- GitHub personal access token、Travis CI token

### 步驟

```bash
git clone https://github.com/MilesChou/devpulse.git
cd devpulse
composer install
cp .env.example .env
php artisan key:generate
```

## 初次設定

### 1. 設定資料庫

預設使用 SQLite，無需額外服務。要切 PostgreSQL，編輯 `.env`：

```env
DB_CONNECTION=pgsql
DB_URL=postgresql://user:password@host:5432/dbname
```

或拆開設定 `DB_HOST` / `DB_PORT` / `DB_DATABASE` / `DB_USERNAME` / `DB_PASSWORD`。

### 2. 設定外部 API token

在 `.env` 填入：

```env
GITHUB_TOKEN=ghp_xxxxx
TRAVIS_TOKEN=xxxxx
```

取得方式：
- **GitHub**：<https://github.com/settings/tokens>（需 `repo` 與 `read:user` scope）
- **Travis**：登入 <https://app.travis-ci.com> → Account Settings → API authentication

### 3. 跑 migration

```bash
php artisan migrate
```

### 4. 建立你的觀測群體（group）

```bash
# 建一個 group
php artisan devpulse:group:create my-team --description="My Team"

# 加 repo 到 group
php artisan devpulse:repo:add my-team your-org/your-repo

# 加成員到 group
php artisan devpulse:member:add my-team alice "Alice Chen"
```

詳細的 group / member / repo 設定流程請見 [docs/group-setup.md](docs/group-setup.md)。

## Quick Start：跑第一份月報

```bash
# 撈資料（透過 GitHub / Travis API 寫入 DB；已過月份會走 cache，--force 可繞過）
php artisan devpulse:fetch my-team 2026-04

# 產出 markdown 報告（不指定 --output 則印到 stdout）
php artisan devpulse:report 2026-04 --group=my-team --output=report.md
```

支援的 command：

```bash
php artisan devpulse:group:create <slug> [--description=...]
php artisan devpulse:repo:add <group-slug> <owner/name>
php artisan devpulse:member:add <group-slug> <github-account> <display-name>
php artisan devpulse:fetch <group-slug> <Y-m> [--force]
php artisan devpulse:report <Y-m> --group=<slug> [--output=<path>]
```

## 開發

```bash
make all        # lint + phpstan + test
make lint       # 只跑 phpcs
make stan       # 只跑 phpstan
make test       # 只跑 phpunit
```

## 文件

- [docs/group-setup.md](docs/group-setup.md) — group / member / repo / human signals 設定
- [docs/migration-from-prototype.md](docs/migration-from-prototype.md) — 什麼時候可以 retire Python prototype
- [openspec/changes/propose-devpulse/](openspec/changes/propose-devpulse/) — 完整 spec、design decisions、tasks

## 技術棧

- Laravel 13（PHP 8.3+）
- PostgreSQL（開發階段可用 SQLite）
- Saloon（HTTP client）、Carbon（datetime）
- 對照來源：Python prototype `ci_analysis`（保留為 golden output）

## License

MIT — 詳見 [LICENSE](LICENSE)
