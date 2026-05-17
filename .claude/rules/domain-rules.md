---
paths:
  - "src/**"
---

# Domain rules

`src/` 是 DevPulse 的純 domain 層，與 Laravel framework 完全解耦。所有業務概念（VO、Entity、Port interface、Domain Service）都住在這裡；framework 適配（Eloquent Model、HTTP Controller、Filament Resource、Saloon Connector）一律放 `app/`。

任何要動 `src/` 的修改都必須先讀過這份規則。違反規則的 PR 要先還掉技術債才能 merge。

---

## 1. 目錄結構

```
src/
├── DevPulse/        # 業務 namespace（composer psr-4: DevPulse\）
│   ├── Ci/          # 持續整合領域（Build, BuildStatus, BuildTrigger, CiProvider port…）
│   ├── Shared/      # 跨領域共用 VO（CommitSha, MonthRange, RepoId, RepoFullName…）
│   └── Vcs/         # 版本控制領域（PullRequest, Author, Platform, Factory…）
└── Tests/           # 對應的 domain test（composer psr-4: Tests\Domain\）
    ├── Ci/
    ├── Shared/
    └── Vcs/
```

| 目錄 | namespace | 用途 |
| --- | --- | --- |
| `src/DevPulse/` | `DevPulse\…` | domain 程式碼 |
| `src/Tests/` | `Tests\Domain\…` | domain 程式碼的測試，**鏡像** `src/DevPulse/` 的子目錄結構 |

**Domain test 跟 domain 程式碼放在 `src/` 下一同管理**：要找 `src/DevPulse/Vcs/PullRequest.php` 的 test，就去 `src/Tests/Vcs/PullRequestTest.php` 找，不要去 `tests/` 翻。
`tests/` 只放 framework 整合測試（Feature/Unit/Aggregation/Persistence/Console/Filament/Infrastructure 這些 Laravel 層次）。

---

## 2. 不可 import 的 namespace（硬性）

**`src/` 下任一檔案只能依賴：**

1. `src/` 內部其他 namespace（`DevPulse\…`）
2. PHP 原生型別與 SPL：`Stringable`、`Generator`、`Iterator`、`Countable`、`ArrayAccess`、`InvalidArgumentException`、`RuntimeException`、`DateTimeImmutable`、`DateTimeInterface`、`DateTimeZone`、`SplObjectStorage`、`WeakMap`、enum、PHP 內建型別…
3. PHPUnit（**僅限** `src/Tests/`，且只用 `PHPUnit\Framework\TestCase`，不引入 Mockery 或其他 mock lib）

**其他全部不可** import，包含但不限於：

- Laravel framework 家族：`App\…`、`Illuminate\…`、`Filament\…`、`Inertia\…`、`Laravel\…`
- 第三方 lib：`Saloon\…`、`Carbon\…`、`Symfony\…`、`Guzzle\…`、`Mockery\…`…
- 任何 PSR interface（`Psr\Log\…`、`Psr\Clock\…`、`Psr\Http\…`…）—— PSR 雖然「中立」，但仍是外部抽象，要用就用 DIP 自己在 domain 定義 interface

**為什麼**：domain 必須能在沒有 Laravel 啟動、甚至沒有 composer 第三方套件的狀態下被測試與推理。一旦 domain 直接抓 `App\Models\Repo`，就跟 Eloquent 雙向耦合；一旦 domain 接 `CarbonImmutable`，未來想換時間 lib 就要動 domain。VO 的意義就是不該因外部變動而動。

PHP 原生型別與 SPL 是 language platform 本身，不算「外部依賴」—— PHP 跑得起來就有它們。

### 需要外部能力時：用依賴反向原則（DIP）

如果 domain 真的需要時間、HTTP、隨機數、ID 產生器…這類能力，**不要**直接 import 外部 lib，而是：

1. 在 `src/` 定義 port interface（domain 自己擁有契約）
2. 在 `app/Infrastructure/` 或 `app/` 寫 adapter 實作該 interface
3. 由 caller（framework 層）在組裝時注入 adapter

```php
// ✅ src/DevPulse/Shared/Clock.php — domain 自己定義契約
namespace DevPulse\Shared;

interface Clock
{
    public function now(): \DateTimeImmutable;
}

// ✅ app/Infrastructure/Clock/CarbonClock.php — adapter 在 framework 層
namespace App\Infrastructure\Clock;

use DevPulse\Shared\Clock;
use Carbon\CarbonImmutable;

final class CarbonClock implements Clock
{
    public function now(): \DateTimeImmutable
    {
        return CarbonImmutable::now()->toDateTimeImmutable();
    }
}

// ✅ src/DevPulse/Vcs/SomeService.php — domain 只認識自己的 interface
namespace DevPulse\Vcs;

use DevPulse\Shared\Clock;

final readonly class SomeService
{
    public function __construct(private Clock $clock) {}
}
```

```php
// ❌ src/DevPulse/Vcs/SomeService.php — domain 直接 import Carbon
namespace DevPulse\Vcs;

use Carbon\CarbonImmutable;  // 違反，請改用 Clock interface

final readonly class SomeService
{
    public function now(): CarbonImmutable { /* … */ }
}
```

Port interface 接受 / 回傳的型別只能是 domain VO 或 PHP 原生 type，不可漏出第三方 type（不能回 `CarbonImmutable`，要回 `DateTimeImmutable`；不能回 `Eloquent\Collection`，要回 `Generator` 或 `array`）。

---

## 3. 允許的內部 import

`src/DevPulse/` 內部可以自由相互 import。建議方向：

- `Vcs/` 與 `Ci/` 可以 import `Shared/`
- `Shared/` 不要 import `Vcs/` 或 `Ci/`（保持單向依賴）
- `Vcs/` 與 `Ci/` 之間互相 import 沒問題，但要想清楚是不是該抽 `Shared/`

```php
// ✅ OK：Vcs 用 Shared
namespace DevPulse\Vcs;
use DevPulse\Shared\RepoId;

// ❌ NG：Shared 反向依賴 Vcs
namespace DevPulse\Shared;
use DevPulse\Vcs\PullRequest;  // 應該把這個概念抽進 Shared，或重新設計
```

---

## 4. 容許的程式碼種類

`src/DevPulse/` 應該只有下列幾類：

### Value Object（最常見）

不可變、由建構式驗證、`equals()` 比值不比 reference。

```php
namespace DevPulse\Shared;

final readonly class RepoId implements Stringable
{
    public function __construct(public string $value)
    {
        if (! preg_match('/^[0-9A-HJKMNP-TV-Za-hjkmnp-tv-z]{26}$/', $value)) {
            throw new InvalidArgumentException("RepoId must be a 26-char ULID (got `{$value}`)");
        }
    }

    public function __toString(): string
    {
        return $this->value;
    }
}
```

- `final readonly` 預設
- 用建構式驗證，**不要**做 setter / mutator
- 失敗丟 `InvalidArgumentException`，訊息要含實際值方便 debug
- 同概念暴露的型別比較方法（如 `equals`）放在 VO 自己

### Entity / Aggregate

有 identity、可變狀態（仍建議 immutable + `with*()` 風格）。

```php
namespace DevPulse\Vcs;

final readonly class PullRequest
{
    public function __construct(
        public PullRequestId $id,
        public RepoId $repoId,
        public PullRequestNumber $number,
        // …
    ) {}

    public function status(): PullRequestStatus { /* … */ }
    public function isDraft(): bool { /* … */ }
}
```

### Enum

業務狀態用 PHP `enum`，搭配 `: string` backing。

```php
namespace DevPulse\Vcs;

enum PullRequestStatus: string
{
    case Open = 'open';
    case Merged = 'merged';
    case Closed = 'closed';

    public static function fromGitHubState(string $state, ?string $mergedAt): self { /* … */ }
}
```

### Port（介面）

對外部系統的契約，由 `app/Infrastructure/` 的 adapter 實作。

```php
namespace DevPulse\Ci;

interface CiProvider
{
    /** @return Generator<int, Build> */
    public function listBuildsInMonth(RepoFullName $repo, MonthRange $month): Generator;
}
```

- Port 接受 / 回傳的全部是 domain VO，不能漏出 framework type（不能回 `Eloquent\Collection`，要回 `Generator` 或 `array`）

### Factory

從外部資料（GitHub payload、Travis payload…）建構 domain VO。

```php
namespace DevPulse\Vcs\Factory;

final class GitHubPullRequestFactory
{
    /** @param array<string, mixed> $raw */
    public static function fromGitHubRaw(array $raw, string $repoId, PullRequestId $id): PullRequest { /* … */ }
}
```

- 入參用原始 array / string，**不要**接 framework Request
- 失敗丟 `InvalidArgumentException`
- 純 static method 或 final class，**不要**注入 service

### Domain Service / Filter

無狀態的 domain 行為。

```php
namespace DevPulse\Vcs\Filter;

final class BotFilter
{
    public function isBotPullRequest(PullRequest $pr): bool { /* … */ }
}
```

---

## 5. 不該出現在 `src/` 的東西

| 反例 | 該放哪 |
| --- | --- |
| Eloquent Model | `app/Models/` |
| Repository（會碰 DB 的） | `app/Persistence/Repository/` |
| Mapper（VO ↔ array of Eloquent attributes） | `app/Persistence/Mapper/` |
| HTTP / Saloon Request / Connector | `app/Infrastructure/` |
| Filament Resource / Form / Table | `app/Filament/` |
| Console Command | `app/Console/Commands/` |
| Job / Listener / Event（Laravel）| `app/Jobs/`、`app/Listeners/`、`app/Events/` |
| 直接使用 `env()`、`config()`、Facade、`app()`、`resolve()` | framework 端注入 |
| 直接 instantiate Carbon facade 取「現在」 | 從外部用 `?CarbonImmutable $now = null` 注入，方便測試 |

---

## 6. 命名規約

- VO / Entity / Factory：**單數**名詞（`Build`，不是 `Builds`）
- Enum：以名詞表達狀態類別（`BuildStatus`、`PullRequestStatus`）
- Port interface：**不要**加 `Interface` / `Port` 後綴，直接用領域語言（`CiProvider`，不是 `CiProviderInterface`）
- Factory：`<X>Factory`，且只能有 `fromXxxRaw()` / `fromXxxPayload()` 這類 static 方法
- 反 anemic：方法盡量是動詞（`isDraft()`、`status()`），不要把 VO 寫成只有 getter 的容器

---

## 7. 測試

`src/Tests/` 鏡像 `src/DevPulse/` 結構：

```
src/DevPulse/Vcs/PullRequest.php          # 被測檔
src/Tests/Vcs/PullRequestSummaryTest.php  # 對應 test
```

- 用 `PHPUnit\Framework\TestCase`，**不要**繼承 Laravel 的 `Tests\TestCase`（避免 boot framework）
- 不可 `use RefreshDatabase`、不可 `Artisan::call()`、不可碰 HTTP
- 純 in-memory：直接 `new VO(...)`，assert 行為
- Test data 用 ULID / commit sha 等真值，不要 hardcode `1` / `2`（會被 RepoId regex 擋掉）

---

## 8. 重構指引

### Smell：framework 漏進來了

```php
// ❌ 在 src/DevPulse/Vcs/PullRequest.php 看到
use Illuminate\Support\Collection;
use App\Models\Repo;
```

**動作**：那個概念屬於 framework 層，把它搬到 `app/`；或抽出一個 domain VO 取代之。

### Smell：domain 要呼叫 DB

```php
// ❌ 在 src/ 想 query DB
Repo::where(...)->get();
```

**動作**：定義一個 Port interface（例如 `RepoLookup`），由 `app/Persistence/` 寫 adapter 注入。

### Smell：VO 變胖到開始放跨領域邏輯

```php
class PullRequest {
    public function fetchLatestReviews(GitHubApi $api): array { /* … */ }
}
```

**動作**：拆出 Domain Service，VO 不應該負責去抓資料。

### Smell：Factory 的 static method 越來越多分支

**動作**：考慮拆 multi factories（`GitHubPullRequestFactory`、`GitLabPullRequestFactory`…），或抽 `PullRequestFactory` interface。

---

## 9. PR Checklist

要動 `src/` 的 PR，self-review 時跑一遍：

- [ ] 跑下面這條 grep，輸出應該只有 `DevPulse\…`、PHP 原生型別、`PHPUnit\Framework\TestCase`：
  ```sh
  grep -rhn "^use " src/ | sort -u
  ```
  看到 `App\`、`Illuminate\`、`Filament\`、`Saloon\`、`Carbon\`、`Symfony\`、`Psr\` 等都是違規
- [ ] 新增的類別有放對 namespace（`DevPulse\…` 在 `src/DevPulse/`、`Tests\Domain\…` 在 `src/Tests/`）
- [ ] VO 是 `final readonly`、建構式驗證、失敗丟 `InvalidArgumentException`
- [ ] Port 的入參 / 回傳都是 domain VO 或 native PHP type
- [ ] Test 用 `PHPUnit\Framework\TestCase`，沒繼承 `Tests\TestCase`
- [ ] `phpunit src/Tests/` 全綠（domain test 應該可以脫離 framework 跑）
