## ADDED Requirements

### Requirement: 取得單月 CI 失敗率月報

使用者 MUST 能用一個指令取得指定月份的「成員 × repo」失敗率報告，並在 Overall 欄位看到與上月對比。

#### Scenario: 跑出月報

- **WHEN** 使用者執行 report 指令並指定月份
- **THEN** 終端機輸出含成員 × repo 的失敗率表格，Overall 欄位附對比箭頭

### Requirement: 取得 PR review latency 月報

使用者 MUST 能取得 PR review latency 的月報，並依 PR 規模切片，看到「不同規模 PR 的 review 等待時間是不是合理」。

#### Scenario: review latency 報告

- **WHEN** 使用者執行 review latency 報告
- **THEN** 輸出含 size bucket（XS/S/M/L/XL）切片的 latency 表格，並對比上月

### Requirement: 取得每日 build 時間趨勢圖

使用者 MUST 能在月報中看到每日 build 時間的視覺化趨勢，方便察覺 CI 變慢。

#### Scenario: 每日 build 時間趨勢

- **WHEN** 使用者跑月報
- **THEN** 月報含每日 build 時間趨勢圖（文字或 ASCII 即可）

### Requirement: 取得失敗 build 清單

使用者 MUST 能在月報中看到當月所有失敗 / 錯誤 build 的清單，每筆能點到原始 build 頁面或對應 PR。

#### Scenario: 失敗 build 列表

- **WHEN** 使用者跑月報
- **THEN** 月報含失敗 build 清單，每筆有可點選的連結與 commit author、PR 資訊

### Requirement: 月報能直接寫入檔案而非只印到螢幕

使用者 MUST 能選擇把月報寫入檔案（例如要進 wiki 或 commit 進團隊 repo），而不是每次都要從 stdout copy paste。

#### Scenario: 寫入檔案

- **WHEN** 使用者執行 report 指令並指定輸出檔路徑
- **THEN** 系統把月報寫入該路徑，不需手動 redirect

### Requirement: 數值結果與 prototype 一致以驗證重寫正確性

使用者 SHOULD 在重寫過渡期，能對同月份、同設定，比對新版（PHP）與既有 prototype（Python ci_analysis）的數值是否一致，做為「重寫沒寫錯」的驗證手段。

#### Scenario: 兩版本結果對得起來

- **WHEN** 使用者對同一個月份在兩個系統上跑相同 profile
- **THEN** 兩個系統的失敗率、PR review latency、build 計數等核心數值相等

