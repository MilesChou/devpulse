## ADDED Requirements

### Requirement: 取得指定月份的 PR 清單

使用者 MUST 能指定一個 repo 與一個月份，系統就能列出該月被建立的所有 PR（含已合併、已關閉、被 reject、仍在 draft 的）。

#### Scenario: 撈取單月 PR

- **WHEN** 使用者指定 `repo="owner/name"` 與 `month="2026-04"`
- **THEN** 系統提供該月內建立的全部 PR 清單

### Requirement: 取得 PR 的關鍵時間點與規模資訊

使用者 MUST 能取得每個 PR 用於計算 review latency 與分桶的資訊：誰開的、何時 mark ready、何時收到第一個 review、是否合併、改動行數。

#### Scenario: 取得 PR 詳細資料

- **WHEN** 使用者要求某個 PR 的詳細資料
- **THEN** 系統提供 author、ready 時間、第一次 review 的時間（若有）、改動行數、目前狀態（開放／關閉／合併）

### Requirement: 把 commit 對應到貢獻者

使用者 MUST 能由 CI build 對應到的 commit，反查回該 commit 的作者，以便將 build 對應到團隊成員。

#### Scenario: commit 對應到 GitHub 帳號

- **WHEN** 使用者提供一組 commit
- **THEN** 系統提供每個 commit 對應的 GitHub 使用者帳號（找不到對應的可省略）

### Requirement: 排除自動化機器人的活動

使用者 MUST 能設定要排除的 bot 帳號清單，讓 bot 開的 PR 與 bot 留的 review 不會干擾統計（例如 dependabot、Copilot 自動 review）。

#### Scenario: dependabot PR 不計入

- **WHEN** 某 PR 的 author 在使用者設定的 bot 清單中
- **THEN** 該 PR 不出現在後續的 PR review latency 與 size 分桶統計裡

### Requirement: 重複查詢同樣資料不應一直打外部服務

使用者 MUST 能在重複跑同樣月份、同樣 repo 的查詢時，不需每次都實際打外部 API，避免速度慢與額度浪費。

#### Scenario: 第二次查詢命中快取

- **WHEN** 使用者第二次查詢同樣的 repo + 月份
- **THEN** 系統使用先前的結果，不重新打外部服務

### Requirement: 外部服務暫時不可用時不應整批中斷

使用者 MUST 在外部服務暫時忙碌（rate limit、503 等）時，看到系統自動退避並重試，而不是整批撈取直接失敗結束。

#### Scenario: 遇到限流仍能完成批次

- **WHEN** 撈取過程中遇到外部服務回應限流
- **THEN** 系統自動等待與重試，最終仍完成這次批次（或在多次重試後給出明確錯誤）

