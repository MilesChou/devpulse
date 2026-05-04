## ADDED Requirements

### Requirement: 維護自己的團隊成員清單

使用者 MUST 能維護一份成員清單，每個成員有顯示名稱與對應的 GitHub 帳號，讓報告能把 commit / PR 對應到「人名」而非 GitHub login。

#### Scenario: 報告顯示人名而非帳號

- **WHEN** 使用者跑月報
- **THEN** 報表的成員欄是設定中的顯示名稱（例如「Member1」），不是 GitHub login（例如「user-1」）

### Requirement: 不同團隊或場景可以用不同 profile

使用者 MUST 能設定多個 profile，每個 profile 含不同的 repos 與成員，讓「自己團隊」、「友隊」、「未來其他部署場景」可以分開觀測。

#### Scenario: 切換 profile 看不同團隊

- **WHEN** 使用者跑月報並指定不同 profile
- **THEN** 系統依該 profile 的 repos 與 members 統計，不混到別的 profile

### Requirement: 工具與特定組織解耦

使用者 MUST 在初次拿到這個工具時，看到的範例設定不含任何特定組織資料（沒有真實的組織名、沒有真實的成員姓名），方便重複部署於不同團隊、避免敏感資訊外洩。

#### Scenario: 初始範本是中性的

- **WHEN** 使用者第一次取得工具並複製範例設定
- **THEN** 範例中只有 placeholder（例如 `your-org/your-repo`、`Member1`），不含真實組織或人名

### Requirement: 排除自動化 bot 的設定

使用者 MUST 能設定要排除哪些 bot 帳號（例如 dependabot、Copilot 自動 review），預設清單已包含常見 bot。

#### Scenario: 預設就排除常見 bot

- **WHEN** 使用者初次安裝後跑月報
- **THEN** dependabot、Copilot 自動 review 等預設不會干擾統計

#### Scenario: 使用者可加新 bot

- **WHEN** 使用者把新 bot 加進排除清單
- **THEN** 後續跑月報該 bot 的活動不再計入

### Requirement: PR 規模分類可調整

使用者 MUST 能調整 PR 規模分類的邊界（XS/S/M/L/XL 各自的行數上限），因為不同團隊對「大 PR」的定義不同。

#### Scenario: 沿用預設

- **WHEN** 使用者未調整邊界
- **THEN** 系統使用內建預設值

#### Scenario: 自訂邊界

- **WHEN** 使用者改 XS 上限為 100
- **THEN** 後續分類採用 100 為界

### Requirement: 失敗訊號規則可依 repo 設定

使用者 MUST 能為每個 repo 設定「哪些 log 字串組合代表是 human error」（例如 phpcs 失敗、phpunit 失敗），讓系統能自動分類失敗原因。

#### Scenario: 設定 lint 規則

- **WHEN** 使用者為某 repo 設定「同時出現 `make phpcs` 與 `phpcs.xml` 即為 lint 失敗」
- **THEN** 該 repo 的失敗 build 若 log 同時含這兩個字串，會被標為 lint 類失敗

### Requirement: API 憑證透過環境變數提供

使用者 MUST 透過環境變數提供外部 API 憑證（GitHub token、Travis token），不能硬編碼於設定檔內，避免不小心被 commit。

#### Scenario: 缺 token 給明確錯誤

- **WHEN** 使用者未設定必要 token 就跑指令
- **THEN** 系統印出明確錯誤訊息（缺哪個 token），而不是去打 API 才報模糊錯誤

