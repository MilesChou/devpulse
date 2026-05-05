## ADDED Requirements

### Requirement: 支援多個 CI 服務不需改變使用習慣

使用者 MUST 能在不同 repo 使用不同 CI 服務的情況下（例如有些 repo 用 Travis、有些用 GitHub Actions），仍以相同的指令與參數查資料，不需要記住「這個 repo 要用哪個指令」。

#### Scenario: 不同 CI 服務、相同指令

- **WHEN** 使用者對 repo A（Travis）與 repo B（GitHub Actions）查同一個月的 CI 失敗率
- **THEN** 兩個 repo 都用同一個指令、回傳相同格式的結果

### Requirement: 第一版至少實作一個 CI 服務並提供 Travis CI

使用者 MUST 在第一版就能查 Travis CI 的資料，因為這是現有 prototype 已驗證的環境；系統 MUST 至少實作一個具體 CI 服務的支援。

#### Scenario: Travis 可用

- **WHEN** 使用者設定 Travis API token 並指定 Travis 上的 repo
- **THEN** 系統能撈到 build 紀錄並計算指標

### Requirement: 未來新增 CI 服務不應強迫使用者重設定

使用者 SHOULD 在系統將來新增支援其他 CI 服務（例如 GitHub Actions、CircleCI）時，不需要遷移既有的設定，原本能用的 repo 仍維持原有設定能繼續用。

#### Scenario: 加入新 CI 服務不影響舊 repo

- **WHEN** 系統未來新增 GitHub Actions 支援
- **THEN** 原本設定為 Travis 的 repo 不需改設定即可繼續工作

### Requirement: 取得指定月份的 CI 執行紀錄

使用者 MUST 能指定 repo 與月份，看到該月所有 CI 執行紀錄（成功、失敗、取消、錯誤都包含）。

#### Scenario: 撈取單月 CI 紀錄

- **WHEN** 使用者指定 repo 與月份
- **THEN** 系統提供該月所有 CI 執行紀錄，含每筆的狀態（成功／失敗／錯誤／取消）、開始時間、執行時長、對應的 commit

### Requirement: 區分不同性質的 CI 執行

使用者 MUST 能區分一筆 CI 執行是「PR 過程的執行」、「合併進主幹後的執行」、還是「部署用的執行」，以便排除非個人責任的失敗。

#### Scenario: PR 階段執行可被識別

- **WHEN** 使用者檢視某筆 CI 執行
- **THEN** 系統能告訴使用者：這是不是 PR 觸發、是不是合併後的執行、是不是部署流程

#### Scenario: 預設只算 PR 階段執行

- **WHEN** 使用者統計失敗率而沒額外指定
- **THEN** 系統預設不把「合併後」與「部署」執行算進去（因為這些不反映個人 PR 品質）

### Requirement: 取得失敗執行的詳細紀錄

使用者 MUST 能看到單一失敗 CI 執行的完整 log，以便判斷失敗原因。

#### Scenario: 取得失敗 log

- **WHEN** 使用者要看某筆失敗 CI 執行的細節
- **THEN** 系統提供該執行的完整 log 文字

### Requirement: 已過完的月份不該重打外部服務

使用者 MUST 在重複查詢已過完月份的資料時，看到立即回應而不是重新從外部服務撈一次。

#### Scenario: 撈過去月份用快取

- **WHEN** 使用者查詢的月份已經結束（例如現在 2026-05、查 2026-04）且之前撈過
- **THEN** 系統直接從本地資料回應，不再打 CI 服務

#### Scenario: 當月仍可重撈

- **WHEN** 使用者查詢的月份仍進行中（例如現在 2026-05、查 2026-05）
- **THEN** 系統允許重新撈取以反映新進來的執行紀錄

