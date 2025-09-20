# Solver ↔ Challenger 程式互動說明

本文以原始碼為依據，說明 Challenger 與 Solver 間的請求/回呼流程、簽章驗證、資料模型、重試與健康檢查等互動細節，並以關鍵程式檔案路徑輔助對照。

---

## 元件與責任分工

- Challenger（發題方）
  - 建立挑戰（包含正確答案與驗證規則，答案不外傳）
  - 將挑戰推送給 Solver（HTTP POST /solve）
  - 接收 Solver 回呼（HTTP POST /callback/{challenge_id}）
  - 依本地驗證規則驗證答案並保存結果
  - 主要檔案：
    - `cmd/challenger/main.go`（HTTP 伺服器、路由、middleware 掛載）
    - `internal/challenger/service.go`（送題、處理回呼、驗證答案、記錄結果/審計）
    - `pkg/db/challenger.go`（Challenger DB schema 與存取）

- Solver（解題方）
  - 接收挑戰（/solve），入列 DB 後由 worker pool 處理
  - 完成後以回呼通知 Challenger（/callback/{challenge_id}）
  - 支援重試與指數退避（含 jitter）
  - 主要檔案：
    - `cmd/solver/main.go`（HTTP 伺服器、路由、middleware 掛載、啟停 worker）
    - `internal/solver/service.go`（處理 /solve、送回呼）
    - `internal/solver/worker.go`（佇列輪詢、解題、重試與狀態更新）
    - `pkg/db/solver.go`（Solver DB schema 與存取）

- 共同元件
  - `pkg/api/middleware.go`：請求日誌、大小限制、HMACAuth、CORS、HTTPSOnly、中健康檢查
  - `pkg/auth/hmac.go`：HMAC-SHA256 簽章/驗證與 Authorization 標頭解析
  - `pkg/models/models.go`：Solve/Callback 請求與回應、DB 模型
  - `pkg/validator/validator.go`：答案驗證規則（ExactMatch / NumericTolerance / Regex）
  - `pkg/config/config.go`：設定載入、密鑰映射與常用 getter

---

## 認證與安全機制

- HMAC 簽章（`pkg/auth/hmac.go`）
  - Canonical String：`METHOD\nPATH\nTIMESTAMP\nNONCE\nSHA256(body)`
  - 標頭格式：`Authorization: RCS-HMAC-SHA256 keyId=...,ts=...,nonce=...,sig=...`
  - 時鐘容忍：預設 ±300 秒（`config.ClockSkewSeconds` 可調）
  - 常數時間比較：`hmac.Equal`

- Nonce 防重放（`pkg/api/middleware.go` + `pkg/db/*`）
  - Middleware 於驗章前後分別檢查與保存 nonce（DB 表 `seen_nonces`）
  - 背景清理：每小時清除超過 2×時間窗的舊 nonce（`cmd/*/main.go` 中 `cleanupNonces`）

- 其他保護
  - 請求大小限制：5MB（`SizeLimit`）
  - CORS：預設允許 `*`（開發友善，生產可收緊）
  - HTTPSOnly：可依 `X-Forwarded-Proto` 導向至 https（生產應在受信代理後使用）

---

## 互動流程一：Challenger → Solver（送出挑戰）

1) 建立挑戰（只在 Challenger 端保存答案與驗證規則）
- `internal/challenger/service.go: CreateChallenge` 將 `models.Challenge` 寫入 `challenges` 表。

2) 發送挑戰到 Solver `/solve`
- `internal/challenger/service.go: SendChallenge`
  - 拼出 callback URL：`{PUBLIC_CALLBACK_HOST}/callback/{challengeID}`
  - 組裝 `models.SolveRequest{ api_version: "v2.1", challenge_id, problem, output_spec, constraints, callback_url }`
  - `pkg/auth/hmac.go: HMACAuth.CreateAuthHeader("POST", "/solve", body, SolverHMACKeyID, nonce)` 生成簽章
  - 設定 `Content-Type: application/json`、`Authorization`、`X-Request-ID` 後 `POST {solver}/solve`

3) Solver 驗證與入列
- Router 與 Middleware（`cmd/solver/main.go`）
  - `/solve` 子路由套用 `middleware.HMACAuth` 先行驗證：
    - 解析授權標頭 → 檢查 nonce 是否已見 → 驗證簽章 → 保存 nonce
- Handler（`internal/solver/service.go: HandleSolve`）
  - 解析 `SolveRequest`，檢查 `api_version == "v2.1"`、`challenge_id` 必填
  - 驗證 `callback_url`（必須 https 開頭）
  - 若 `GetChallenge(challenge_id)` 已存在，直接回覆 202 與既有 `solver_job_id`
  - 否則建立 `models.PendingChallenge{status: "pending"}` 寫入 `pending_challenges`
  - 回覆 202：`models.SolveResponse{ message: "Challenge accepted", solver_job_id }`

4) 設定與密鑰對應
- 雙方的 HMAC 秘鑰從 `pkg/config/config.go` 載入：
  - Challenger 啟動時使用 `cfg.GetChallengerSecrets()`
  - Solver 啟動時使用 `cfg.GetSolverSecrets()`
  - 若使用 `SHARED_SECRET_KEY`，兩側會將 `chal` 與 `solver` 的 keyId 都映射到同一把密鑰。

時序摘要（送題）：

```
Challenger                      Solver
    |  Build SolveRequest          |
    |  HMAC Sign (key=solver)      |
    |-- POST /solve -------------->|
    |                              |  HMAC middleware: parse/nonce/signature
    |                              |  HandleSolve: validate/store pending
    |<------------- 202 Accepted --|
```

---

## 互動流程二：Solver → Challenger（回呼結果）

1) Solver 取出並處理挑戰
- Worker Pool 啟動（`internal/solver/worker.go: WorkerPool.Start`）
  - Dispatcher 每 5 秒抓取 `pending_challenges`（`status='pending'` 或 `processing` 且 `next_retry_time<=now`）
  - 將任務丟入 `jobQueue`，Workers 消費並呼叫 `processChallenge`
- `processChallenge`：
  - `UpdateChallengeStatus(..., status='processing')`
  - `solveChallenge`（MVP 模擬：captcha/math/text）
  - 組裝 `models.CallbackRequest{ api_version:"v2.1", challenge_id, solver_job_id, status, answer?, metadata? }`

2) 送出回呼 `/callback/{challenge_id}`（含重試）
- `internal/solver/worker.go: sendCallbackWithRetry`
  - 失敗重試條件：網路錯誤、429、5xx；最大 6 次，指數退避（500ms 基底、上限 30s、jitter 0.85–1.15）
  - 每次重試前更新 DB：`UpdateChallengeStatus(..., attempt_count, next_retry_time)`
- 實際送出回呼（`internal/solver/service.go: SendCallback`）
  - 將 `CallbackRequest` 序列化為 JSON
  - Canonical PATH 僅包含路徑（例如 `/callback/{challenge_id}`），不含主機與查詢字串
  - 使用 Challenger 的 keyId 簽章：`CreateAuthHeader("POST", callbackPath, body, ChalHMACKeyID, nonce)`
  - `POST {callback_url}` 並回傳 HTTP 狀態碼供重試決策

3) Challenger 驗證與保存結果
- Router 與 Middleware（`cmd/challenger/main.go`）
  - `/callback/{challenge_id}` 子路由套用 `middleware.HMACAuth`：
    - 解析授權標頭 → 檢查 nonce 是否已見 → 驗證簽章 → 保存 nonce
- Handler（`internal/challenger/service.go: HandleCallback`）
  - 解析 `CallbackRequest`，比對 URL 與 Body 的 `challenge_id`
  - 從 DB 讀出挑戰並以 `validator` 依本地規則驗證答案（僅當 `status == "success"` 且 `answer` 存在）
  - 建立 `models.Result{ challenge_id, request_id(X-Request-ID), solver_job_id, status, answer, is_correct, metadata, created_at }`
  - 保存結果（具有冪等保護）：`SaveResultWithDuplicateCheck` 使用 `INSERT OR IGNORE` 並依據 `(challenge_id, request_id)` 唯一鍵避免重複
  - 非重複時記錄 webhook 審計（標頭、body hash、狀態碼）
  - 回覆 `models.CallbackResponse{ received:true, challenge_id, duplicate }`

時序摘要（回呼）：

```
Solver(worker)                 Challenger
    |  Build CallbackRequest       |
    |  HMAC Sign (key=chal)        |
    |-- POST /callback/{id} ------>|
    |                               | HMAC middleware: parse/nonce/signature
    |                               | HandleCallback: validate + store result
    |<-------------- 200 OK --------|
    |                               | (optional) webhook audit record
```

備註：結果冪等性以 `(challenge_id, X-Request-ID)` 為鍵；`X-Request-ID` 由呼叫端（此處為 Solver 的回呼請求）生成與傳遞。

---

## 資料模型（跨服務）

- 請求/回應（`pkg/models/models.go`）
  - SolveRequest：`api_version, challenge_id, problem(json), output_spec(json), constraints, callback_url`
  - SolveResponse：`message, solver_job_id`
  - CallbackRequest：`api_version, challenge_id, solver_job_id, status, answer?, error_code?, error_message?, metadata?`
  - CallbackResponse：`received, challenge_id, duplicate`

- DB（重點欄位）
  - Challenger：
    - `challenges`（含 validation_rule 與答案，僅存於 Challenger）
    - `results`（`UNIQUE(challenge_id, request_id)` 用於冪等）
    - `webhooks`（回呼審計）
    - `seen_nonces`（防重放）
  - Solver：
    - `pending_challenges`（工作佇列：status、attempt_count、next_retry_time）
    - `seen_nonces`（防重放）

---

## 端點總覽

- Challenger
  - `POST /callback/{challenge_id}`：接收 Solver 回呼（需 HMAC）
  - `GET /healthz`：存活檢查
  - `GET /readyz`：DB 連線檢查

- Solver
  - `POST /solve`：接收 Challenger 的送題（需 HMAC）
  - `GET /healthz`、`GET /readyz`：健康檢查
  - `GET /stats`：工作池統計（開發用途）

---

## 關鍵實作對照（檔案/函式）

- 送題（Challenger → Solver）
  - `internal/challenger/service.go: SendChallenge`
  - `pkg/auth/hmac.go: HMACAuth.CreateAuthHeader`
  - `cmd/solver/main.go` 路由 + `pkg/api/middleware.go: HMACAuth` + `internal/solver/service.go: HandleSolve`

- 回呼（Solver → Challenger）
  - `internal/solver/worker.go: processChallenge` → `sendCallbackWithRetry` → `internal/solver/service.go: SendCallback`
  - `cmd/challenger/main.go` 路由 + `pkg/api/middleware.go: HMACAuth` + `internal/challenger/service.go: HandleCallback`

- 防重放與清理
  - `pkg/api/middleware.go: checkNonce/saveNonce`
  - `pkg/db/*: HasSeenNonce/SaveNonce/CleanupOldNonces`
  - `cmd/*/main.go: cleanupNonces`（每小時）

- 驗證邏輯
  - `pkg/validator/validator.go`（ExactMatch / NumericTolerance / Regex）

---

## 備註與邊界

- 簽章的 Canonical String 使用 URL 的 `EscapedPath`（不含查詢字串），若未來介面攜帶 query，需同步調整簽章算法。
- `CORS` 與 `/stats` 端點在開發模式較寬鬆，生產建議收斂權限與來源。
- Nonce 檢查目前採「查詢後保存」流程；多實例可能產生 TOCTOU 競態，實務可改為「寫入即檢測」（例如 `INSERT OR IGNORE` 後依 rows affected 判斷）。

---

以上內容反映現行原始碼的實際互動行為，可配合 `README.md` 與 `ARCHITECTURE.md` 的高階說明對照理解。

