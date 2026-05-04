## ADDED Requirements

### Requirement: 支援多個 CI 服務不需改變使用習慣

使用者 MUST 能在不同 repo 使用不同 CI 服務的情況下（例如有些 repo 用 Travis、有些用 GitHub Actions），仍以相同的指令與參數查資料，不需要記住「這個 repo 要用哪個指令」。

#### Scenario: 不同 CI 服務、相同指令

- **WHEN** 使用者對 repo A（Travis）與 repo B（GitHub Actions）查同一個月的 CI 失敗率
- **THEN** 兩個 repo 都用同一個指令、回傳相同格式的結果

### Requirement: 第一版至少支援 Travis CI

使用者 MUST 在第一版就能查 Travis CI 的資料，因為這是現有 prototype 已驗證的環境。

#### Scenario: Travis 可用

- **WHEN** 使用者設定 Travis API token 並指定 Travis 上的 repo
- **THEN** 系統能撈到 build 紀錄並計算指標

### Requirement: 未來新增 CI 服務不應強迫使用者重設定

使用者 SHOULD 在系統將來新增支援其他 CI 服務（例如 GitHub Actions、CircleCI）時，不需要遷移既有的設定，原本能用的 repo 仍維持原有設定能繼續用。

#### Scenario: 加入新 CI 服務不影響舊 repo

- **WHEN** 系統未來新增 GitHub Actions 支援
- **THEN** 原本設定為 Travis 的 repo 不需改設定即可繼續工作

