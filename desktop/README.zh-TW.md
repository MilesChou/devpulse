# DevPulse 桌面 dashboard

以 Rust 和 [egui](https://github.com/emilk/egui) 寫成的原生 dashboard，用來檢視 DevPulse 蒐集的指標。它是 `devpulse serve` 所提供 JSON API 的唯讀 client：資料蒐集（GitHub、CI provider、資料庫）全部由 Go 服務負責，這個 app 只需要 server URL 和 API token。

> English: [README.md](README.md)

![Dashboard](../docs/images/desktop-dashboard.jpg)

針對每個 repo 和月份範圍，畫面會顯示：

- **KPI 卡片**，對應專案目標：CI 失敗率（理想值 0%）、每個 PR 的 build 次數作為 re-push 的替代指標（理想值 1）、PR 從建立到 merge 的 lead time（理想值 24h），以及 review 等待時間。選擇單一月份時，也會顯示和前一個月相比的變化。
- **PR 尺寸分布**（理想：多數為 XS / S）和**每日 build 時間**。
- **DORA 卡片**：部署頻率、變更前置時間、變更失敗率和恢復時間，附逐月變化。專案目標沒有訂 DORA 的目標值，所以卡片標示的是哪個方向比較好，而不是理想值。如果 server 還不知道 repo 的 default branch，面板會提示執行 `devpulse repo refresh`。
- **12 個月趨勢**：CI 失敗率、PR lead time（avg / p50 / p90）、每週部署次數和變更失敗率，終點為目前選擇的範圍。

## 執行

在存放 DevPulse 資料庫的機器上啟動 API（參考 [`serve`](../docs/commands.zh-TW.md#serve)）：

```bash
DEVPULSE_API_TOKEN=change-me devpulse serve
```

接著編譯並執行 dashboard（需要 Rust 1.95 以上）：

```bash
make desktop-run
```

開啟 **Settings**，輸入 server URL（預設 `http://127.0.0.1:8080`）和 API token，再按 **Save & connect**。**Test** 會同時確認 server 有在運作、token 也被接受。

如果 server 在另一台主機上，`HTTP_ADDR` 必須設成非 loopback 位址，此時一定要設定 `DEVPULSE_API_TOKEN`。API 走的是一般 HTTP，離開信任的網路時，請在前面放一個 TLS reverse proxy（或走 SSH tunnel）。

## 設定存放位置

| 項目 | 位置 |
|---|---|
| API token | 作業系統的 keychain（macOS Keychain、Windows Credential Manager、Linux 的 Secret Service），每個 server URL 各存一筆 |
| Server URL、上次選擇的 repo | 作業系統設定目錄下的 `desktop.json`（macOS 為 `~/Library/Application Support/devpulse/`） |

GitHub 和 CI 的 token 不會傳到 dashboard，只留在 server 上。

以下環境變數用於腳本啟動和開發，只對當次執行有效，不會寫入設定檔或 keychain：

| 變數 | 效果 |
|---|---|
| `DEVPULSE_SERVER_URL` | 這次執行使用的 server URL |
| `DEVPULSE_API_TOKEN` | 這次執行使用的 API token（不讀取 keychain） |
| `DEVPULSE_DESKTOP_CONFIG` | 設定檔的路徑 |

## 開發

```bash
make desktop-test   # cargo test
make desktop-lint   # cargo fmt --check + clippy -D warnings
make desktop        # release 版：desktop/target/release/devpulse-desktop
```

測試會解析 Go API 在 [`internal/http/testdata/`](../internal/http/testdata/) 的 golden 檔案，所以 API 的 JSON 格式一旦改變，除了 Go 的測試，這裡的測試也會失敗。確定要改格式時，用 `go test ./internal/http/ -run Golden -update` 重新產生 golden 檔案，再更新 `src/api.rs` 裡的 Rust 型別。

| 檔案 | 用途 |
|---|---|
| `src/api.rs` | HTTP client 和 API 型別 |
| `src/state.rs` | Dashboard 狀態，以及 API 結果如何更新狀態 |
| `src/kpi.rs` | KPI 卡片的文字和逐月變化 |
| `src/month.rs` | `YYYY-MM` 的月份計算 |
| `src/settings.rs` | 設定檔和 keychain 存取 |
| `src/app.rs` | egui 畫面繪製；API 呼叫在 worker thread 執行 |
