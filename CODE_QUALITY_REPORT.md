# Reverse Challenge System 程式碼品質審視報告

作者：自動化審查助手（基於原始碼靜態閱讀）

本報告針對倉庫結構、程式風格、錯誤處理與日誌、安全性、資料存取層、併發模型、測試與文件等面向進行審視，並提出可執行的改進建議。評估以目前倉庫內容為基礎，未實際啟動服務或執行壓力測試。

## 總覽評語

- 整體架構清晰，資料流與安全邊界（Challenger 保留答案、Solver 僅計算並回傳）定義到位，文件完整（README、ARCHITECTURE、DEPLOYMENT）。
- Go 專案分層良好：`cmd/`（入口）、`internal/`（服務邏輯）、`pkg/`（可重用套件），並有對應測試。日誌、驗證、設定、DB 抽象等模組化設計明確。
- 安全性基礎扎實：HMAC 簽章、時間窗、nonce 防重放、常數時間比對、請求大小限制、HTTPS 檢查等均已涵蓋（MVP 範圍）。
- 測試覆蓋面向核心套件（auth/config/db/validator/api middleware）相對完整，但仍缺少端對端與 worker pool/HTTP handler 的整合與壓力測試。
- 少量一致性/正確性/可維護性問題：
  - 測試與實作對 nonce 重複插入的行為預期不一致（`pkg/db/challenger_test.go` 對 `SaveNonce` 的預期）。
  - middleware 中以 `db` 作為變數名與 package 名相同，造成可讀性降低。
  - 重放防護存在競態（先查再寫的 TOCTOU）。
  - `math/rand` 未設置種子，導致 jitter 與 mock 行為可預測。
  - 權限控制與 CORS 在開發模式較寬鬆（可考慮生產環境收緊）。

整體評價：B+（MVP 架構與實作品質良好，易於強化與擴展）。

## 專案結構與可維護性

- 結構：
  - `cmd/{challenger,solver}`：HTTP 服務入口、路由與生命週期管理。
  - `internal/{challenger,solver}`：業務邏輯（callback 驗證、任務派發、worker pool）。
  - `pkg/{api,auth,config,db,logger,models,validator}`：中介層、認證、設定、資料庫、日誌、資料模型、驗證規則。
  - `examples/`：示例腳本，有助於開發與手動驗證。
  - `AICodingJournal/`：AI 程式開發紀錄與測試摘要文檔。
- 模組邊界：`internal` 與 `pkg` 責任劃分合理，`pkg` 內聚度高，可重用性好。
- 命名與風格：整體符合 Go 習慣，少數區塊可讀性可再提升（如 middleware 內 `switch db := m.db.(type)` 與 package `db` 同名）。

建議：
- 統一中英文命名與註釋風格，避免混雜影響長期維護。
- 將常見常數（如 API 版本 `v2.1`、`MaxRequestSize` 等）集中於單一包或 config 中，降低重複定義風險。

## 錯誤處理與日誌

- 錯誤處理：
  - 以 `fmt.Errorf("...: %w", err)` 方式包裹錯誤，訊息具體，便於追蹤。
  - API 層錯誤回應結構統一（`models.ErrorResponse`），狀態碼與錯誤碼一致性良好。
  - 少數 `json.NewEncoder(...).Encode(...)` 未檢查返回錯誤，實務上影響有限但可補強。
- 日誌：
  - 使用 `zerolog`，並在關鍵請求/資源上附帶 `request_id`、`challenge_id`、`key_id` 等欄位，利於追蹤。
  - 服務啟停、重試、DB 操作失敗等關鍵事件均有紀錄。

建議：
- 在 HTTP 回應寫入（Encode）失敗時補上錯誤處理與記錄。
- 考慮將 request-scoped logger 下放至更多層，以保證跨層一致追蹤（或使用 context 傳遞）。

## 安全性與合規

- 已有：
  - HMAC-SHA256，簽章字串包含 METHOD、PATH、TS、NONCE、SHA256(body)；使用常數時間比較；可配置時間窗。
  - Nonce 防重放：DB 記錄，定期清理；中介層在驗證簽章後嘗試保存 nonce。
  - 請求大小限制（5MB）、CORS、HTTPS-only（依 `X-Forwarded-Proto` 轉導）。
- 風險與改進：
  - 重放防護 TOCTOU：`HasSeenNonce` 與 `SaveNonce` 分離，若多進程/多實例併發，同一 nonce 可能在驗證到保存間隙被重複接受。建議：以單一原子操作落 DB（如 `INSERT ... ON CONFLICT DO NOTHING`/`INSERT OR IGNORE`），並以是否影響列數作為是否重放的判斷來源，將「偵測重放」放到寫入時而非「先查再寫」。
  - HTTPS 檢查依賴 `X-Forwarded-Proto`，需確保只信任受控的反向代理；生產建議在邊緣層終止 TLS，並於應用層校驗可信代理來源。
  - CORS 當前為 `*`，開發友善但生產需收斂；亦可依環境變數配置白名單。
  - Canonical String 未簽入查詢參數（MVP 標註），若未來介面含 query，需定義排序/序列化規則並納入簽章。
  - `stats` 端點未鑑權，可能洩露運行資訊；生產建議加上保護（IP 白名單、Auth、或移除公開暴露）。

## 資料存取層（SQLite）

- 正向面：
  - 啟用 WAL 與 `busy_timeout`，對高併發寫入友好。
  - SQL 參數化，避免 SQL 注入。
  - 適度索引（狀態/時間、nonce 時間）。
  - `ChallengerDB` 與 `SolverDB` 欄位模型清楚，`models` 統一定義 DB/JSON 欄位。
- 可改進：
  - Nonce 表為 `PRIMARY KEY(nonce)`，`SaveNonce` 重複插入會報錯；若業務預期「多次保存同一 nonce 不視為錯誤」，應使用 `INSERT OR IGNORE` 或捕捉違反唯一鍵錯誤後視為成功（這也能支撐「寫入即檢測重放」策略）。
  - DB 操作多為 `Exec/QueryRow`（無 context 版本），可考慮引入 `Context` 版本以支援取消與逾時控制。
  - Readiness 檢查目前以查詢 nonce 近似代替，生產可擴充更嚴謹的探針（如 pragma/寫測試記錄至臨時表等）。

## 併發模型與可靠性

- Worker Pool（`internal/solver/worker.go`）：
  - 具備固定工數、佇列、定時輪詢、指數退避與抖動（jitter），流程完整。
  - 在更新狀態、重試與刪除任務的順序上合理。
- 風險與改進：
  - `math/rand` 未 `Seed`，導致 jitter 與 mock solver 輸出可被預測；建議在啟動時 `rand.Seed(time.Now().UnixNano())`。
  - 併發挑戰重複拾取：目前依據 DB 查詢條件（`status` 與 `next_retry_time`）搭配 worker 將任務轉為 `processing` 降低重複，但在多實例情況仍可能有競態。可考慮在撈取時即以原子 `UPDATE ... WHERE status='pending' ... RETURNING` 形式鎖定，或引入任務租約（lease）欄位。
  - 回呼重試策略完備，但可擴充基於 `Retry-After` 或回應標頭的自適應延遲。

## API 與中介層（Middleware）

- `RequestLogging`、`SizeLimit`、`HMACAuth`、`CORS`、`HTTPSOnly` 中介層劃分清楚、組合合理。
- `HMACAuth` 流程：解析標頭 → 讀入/回填請求體 → 檢查 nonce → 驗證簽章 → 儲存 nonce（錯誤不致命）。

可改進：
- 中介層 `switch db := m.db.(type)` 中的變數名 `db` 與 package `db` 同名，降低閱讀清晰度；可改名為 `database`/`store`。
- `HTTPSOnly` 目前未在 `cmd/*` 中啟用；若有需要強制 HTTPS，可於生產環境掛上。
- 錯誤回應 `json.Encoder.Encode` 建議檢錯並記錄。

## 測試品質

- 已涵蓋：
  - `pkg/auth`：簽章、解析、時間窗、異常路徑。
  - `pkg/validator`：三類規則全面性測試（成功/失敗/異常）。
  - `pkg/config`：預設、覆寫、驗證邏輯、getter。
  - `pkg/db`：CRUD、索引條件、清理、錯誤路徑多數覆蓋。
  - `pkg/api/middleware`：CORS、Size、HMAC 缺失/失敗/成功、HTTPSOnly、健康檢查。
- 缺口：
  - `internal/solver` 與 `internal/challenger` 的 HTTP handler、worker pool 與整體流程尚無端對端整合測試。
  - `pkg/db/challenger_test.go` 對 `SaveNonce` 的重複插入「不報錯」預期與目前實作（PRIMARY KEY）衝突，建議修正測試或調整實作策略（見上方資料庫章節）。
  - 尚未配置持續整合（CI）與靜態分析（golangci-lint/staticcheck），無自動化品質閘。

建議：
- 新增端對端測試：啟動 in-memory/本地服務，發送 `/solve` → worker → 回呼 `/callback`，檢查 DB 狀態與重試行為。
- 導入 CI（GitHub Actions）跑 `go test ./...` 與 linters。
- 若需量化覆蓋率，加入 `-coverprofile` 並於 CI 上傳報表。

## 設定、部署與營運

- 設定：`.env` + `config.Load()` 驗證必要值（`PUBLIC_CALLBACK_HOST` 與密鑰），預設值合理。
- 部署：有 `Dockerfile.*` 與 `docker-compose.yml`，Makefile 工作流完善（build/test/run/example/clean）。
- 觀測性：`zerolog` 結構化日誌，`/healthz`、`/readyz`、`/stats`（開發）提供基本健檢與可觀測切入點。

建議：
- 生產環境將 `stats`、CORS 等收斂為白名單/鑑權。
- 機敏資訊（密鑰）在日誌避免輸出，並檢查第三方依賴版本更新策略。

## 重要問題清單（優先度排序）

1) Nonce 重複插入策略與重放偵測（高）
- 現況：先查 `HasSeenNonce` 再 `SaveNonce`，多實例下可能 TOCTOU；且 `SaveNonce` 對重複插入會報錯，與測試預期不一致。
- 建議：將「偵測重放」合併到「寫入」：`INSERT OR IGNORE`（或等價方案）+ 以「是否成功插入」判斷是否首次見到 nonce；中介層若偵測到非首次可直接拒絕請求（更嚴格）或維持記錄警告（較寬鬆）。同步修正測試預期。

2) 併發鎖定與任務租約（中高）
- 現況：dispatcher 輪詢 + worker 將狀態更新為 `processing`，在多實例或競態場景可能重複拾取。
- 建議：以單 SQL 原子轉換狀態（`pending` → `processing` 並設定租約/到期）或引入分散式鎖（如遷移到 PostgreSQL/Redis 方案）。

3) `math/rand` 可預測性（中）
- 現況：未 `Seed`，導致 jitter 與 mock 行為具可預測性。
- 建議：啟動時 `rand.Seed(time.Now().UnixNano())`。

4) 中介層可讀性與嚴格性（中）
- 變數命名避免遮蔽 package 名；`HTTPSOnly` 與 CORS 在生產環境更嚴格化；簽章可考慮擴充至查詢參數。

5) 測試與自動化（中）
- 新增整合測試與 CI + linter；補齊 encoder 錯誤檢查分支。

## 總結

此專案在架構清晰度、模組化、文件與單元測試覆蓋方面達到良好水準，MVP 所需的安全與可靠性要素基本齊備。若能優先解決 nonce 與重放偵測的競態問題、加強任務鎖定與生產環境保護、補齊整合測試與 CI，自可進一步提升到 A 級品質並更適合多實例/生產部署。
