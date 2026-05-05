## ADDED Requirements

### Requirement: 計算單月「成員 × repo」的 CI 失敗率

使用者 MUST 能看到指定月份內每個成員在每個 repo 的 CI 失敗率，做為個人觀測指標。

#### Scenario: 月失敗率計算

- **WHEN** 使用者指定 group（含成員與 repo）與月份
- **THEN** 系統提供每個 (成員, repo) 組合的：執行總數、失敗數、失敗率

### Requirement: 失敗率預設排除非個人責任的執行

使用者 MUST 在未額外設定時，看到「合併後執行」與「部署執行」自動被排除於失敗率計算外，避免這些失敗誤判為個人問題。

#### Scenario: 預設不算 post-merge

- **WHEN** 使用者未指定包含 post-merge
- **THEN** 合併進主幹後的 CI 執行不計入個人失敗率

#### Scenario: 可手動加回

- **WHEN** 使用者明確要求包含 post-merge
- **THEN** 這些執行被計入

### Requirement: 計算 PR 從 ready 到首次 review 的等待時間

使用者 MUST 能看到每個 PR 從「mark 為 ready」到「收到第一個 review」之間的時數，做為團隊 review 即時性的指標。

#### Scenario: PR 已收到 review

- **WHEN** PR 有 ready 時間且已收到 review
- **THEN** 系統計算兩個時間點的差，以小時為單位

#### Scenario: PR 月底前還沒收到 review

- **WHEN** PR 已 ready 但月底時仍無 review
- **THEN** 系統以月底時間（或當下時間，取較早者）做為等待時間下限

#### Scenario: 仍是 draft 的 PR 不算

- **WHEN** PR 至今仍是 draft（沒 mark ready）
- **THEN** 該 PR 不計入 review latency 統計

### Requirement: 將 PR 依規模分類

使用者 MUST 能看到每個 PR 依改動行數被歸入規模分類（例如 XS / S / M / L / XL），讓 review latency 可以依規模切片觀測（小 PR 的 review 應該更快）。

#### Scenario: 小 PR 分到 XS

- **WHEN** PR 改動 49 行且規模設定為 `XS<50, S<200, ...`
- **THEN** 該 PR 屬於 XS

#### Scenario: 規模設定可由使用者調整

- **WHEN** 使用者調整規模分類邊界
- **THEN** 後續分類採用新邊界

### Requirement: 月對月對比

使用者 MUST 在看當月指標時，同時看到與上月的對比方向（上升／下降／持平）與差值，方便快速判讀趨勢。

#### Scenario: 失敗率上升要可見

- **WHEN** 當月失敗率為 5%、上月為 3%
- **THEN** 結果旁標示「↑ +2%」

### Requirement: 觀察每日 build 時間趨勢

使用者 MUST 能看到每個 repo 每日通過 build 的執行時間，以便察覺 CI 變慢的趨勢。

#### Scenario: 每日 build 時間

- **WHEN** 使用者查某 repo 某月的 daily build duration
- **THEN** 系統提供每日的 build 時間統計（例如 median、最大值或全部 sample）

### Requirement: 計算每個 PR 的 build 重跑次數

使用者 MUST 能看到「平均一個 PR 跑了幾次 build」做為「一次發 PR 就能 merge」的近似指標（理想值 = 1）。

#### Scenario: 同一 PR 跑多次 build

- **WHEN** 某 PR 共觸發 3 次 build
- **THEN** 系統記錄該 PR 的 build 次數為 3，可用於計算月平均

