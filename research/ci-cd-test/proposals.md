## Solution Proposals

### Context
- Request: 實作 CI/CD 自動化管線，觸發條件為當程式碼推播（push）至 `main` 分支，以及向 `main` 發起拉取請求（pull_request）時自動執行測試與驗證。
- Research Source: `research/ci-cd-test/research.md`（第 1 節 CI/CD 現況與 Git 設定、第 2 節 現有單元測試實作與分佈、第 3 節 本地建置與容器化實作、第 4 節 專案指引文件規範、Data Flow 章節）。

---

### Proposal 1 — 輕量多作業單元測試與容器建置管線（Multi-Job Unit Test & Docker Build Pipeline）

- Overview:
  本方案建立單一 GitHub Actions 工作流程，針對 `main` 分支的 push 與 PR 觸發。採用多作業平行矩陣設計，分別在 GitHub Actions Runner 上原生執行 Go 與 .NET 單元測試，並執行 `docker compose build` 驗證所有服務映像檔的建置完整性。
- Key Changes:
  - 新增工作流程定義檔 `.github/workflows/ci.yaml`：
    - **觸發條件**：設定 `on: push: branches: [main]` 與 `on: pull_request: branches: [main]`；加入 `concurrency`（`group: ${{ github.workflow }}-${{ github.ref }}`, `cancel-in-progress: true`），避免同分支/PR 重複堆疊執行。
    - **Job 1 (`go-unit-tests`)**：使用 `actions/setup-go@v5` 配置 Go 1.23，依據 `AGENTS.md:15` 在各服務目錄執行 `go test ./...`，驗證 `src/checkoutservice/money/money_test.go:1-245`、`src/frontend/money/money_test.go:1-245`、`src/frontend/validator/validator_test.go:1-185`、`src/productcatalogservice/product_catalog_test.go:1-101` 與 `src/shippingservice/shippingservice_test.go:1-141`。
    - **Job 2 (`dotnet-unit-tests`)**：使用 `actions/setup-dotnet@v4` 配置 .NET 9.0，依據 `AGENTS.md:16` 執行 `dotnet test src/cartservice/tests/cartservice.tests.csproj`，驗證 `src/cartservice/tests/CartServiceTests.cs:27-160` 與 `src/cartservice/tests/cartservice.tests.csproj:1-21`。
    - **Job 3 (`docker-build`)**：在虛擬機環境呼叫 `docker compose build`，驗證 `compose.yaml:1-123` 宣告之 11 個微服務 Dockerfile（包含 `src/adservice/Dockerfile:1-41` 至 `src/shippingservice/Dockerfile:1-41`）之多階段構建，確保無編譯中斷。
- Trade-offs:
  - 優點：
    1. **快速回饋（Fail Fast）**：Go 與 .NET 單元測試直接於 Runner 原生環境執行，無需啟動容器與網路協同，通常於 1 分鐘內即可完成反饋。
    2. **故障定位直觀**：多 Job 平行架構能讓開發現實於 GitHub Actions 介面一目了然是哪一語言或哪項檢查失敗。
    3. **資源消耗低**：不需要在 CI 環境中拉取與常駐執行全部 11 個服務容器。
  - 缺點與風險：
    1. 僅驗證個別服務單元邏輯與 Dockerfile 編譯完整性，未驗證容器運行時（Runtime）的健康狀態與多服務間的 gRPC 網路資料流。
- Validation:
  - 單元驗證：於本機或容器內依序執行 `go test ./...`（針對 Go 服務）與 `dotnet test src/cartservice/tests/cartservice.tests.csproj`，確認既有單元測試全數通過。
  - 建置驗證：執行 `docker compose build` 確認 11 項服務映像檔皆能構建成功。
  - CI 流程驗證：建立測試分支發起向 `main` 的 PR，確認 GitHub Actions 自動觸發且各 Job 皆呈綠燈；向 `main` 分支推送 commit，確認觸發 push 事件並順利執行完畢。
- Open Questions:
  - `docker-build` 作業是否應設定 `needs: [go-unit-tests, dotnet-unit-tests]`（循序執行），抑或是與單元測試平行執行以最大化速度？

---

### Proposal 2 — 全自動整合驗證管線（All-in-One Compose Test & E2E Smoke Test Pipeline）

- Overview:
  本方案除執行各語言單元測試外，進一步以 `compose.yaml` 為核心執行端對端系統整合測試。在 Runner 上啟動完整的 Docker Compose 服務棧，並實施 HTTP 健康檢查與首頁端對端探針（Smoke Test），確保前端與各後端 gRPC 服務在真實組網與環境變數設定下的運行正常。
- Key Changes:
  - 新增工作流程定義檔 `.github/workflows/ci.yaml`：
    - **觸發條件**：設定 `on: push: branches: [main]` 與 `on: pull_request: branches: [main]`，配置 `concurrency` 控制。
    - **Job 1 (`unit-tests`)**：使用矩陣執行 Go（Go 1.23）與 .NET（.NET 9.0）單元測試（包含 `src/checkoutservice/`、`src/frontend/`、`src/productcatalogservice/`、`src/shippingservice/` 與 `src/cartservice/`）。
    - **Job 2 (`integration-and-smoke-test`)**：
      - 依據 `compose.yaml:1-123` 與 `AGENTS.md:11` 執行 `docker compose up --build -d`。
      - 透過健康檢查機制輪詢等待 `redis-cart`（`compose.yaml:14-18`）與 `frontend`（`compose.yaml:104-105`）就緒。
      - 執行 Smoke Test：使用 `curl -f http://localhost:8080/` 請求前端頁面，驗證 HTTP 200 狀態碼，藉此觸發前端向 `productcatalogservice`（商品讀取）、`currencyservice`（匯率轉換）等 gRPC 通訊。
      - 於 `always()` 步驟呼叫 `docker compose down`（`AGENTS.md:13`）與收集失敗時之容器日誌（`docker compose logs`）。
- Trade-offs:
  - 優點：
    1. **覆蓋度高**：完整驗證 Docker Compose 組網、連接埠暴露（`127.0.0.1:8080:8080`）、各服務環境變數（如 `PRODUCT_CATALOG_SERVICE_ADDR`）與 gRPC 服務間連線。
    2. **高度還原本機開發體驗**：直接驗證專案核心指南 `README.md:35-74` 所描述的標準本機啟動流程。
  - 缺點與風險：
    1. **CI 執行時間較長**：完整建置、下載基礎映像檔並啟動 11 項微服務，總執行時間預計增加至 3–8 分鐘。
    2. **除錯複雜度高**：整合測試失敗時需跨多個容器輸出日誌進行排查。
    3. **Runner 資源壓力**：GitHub 託管之標準 Runner（2-core CPU / 7GB RAM）同時啟動多個容器時，負載較高。
- Validation:
  - 本地模擬驗證：本地執行 `docker compose up --build -d`，使用 `curl -i http://localhost:8080/` 檢查首頁回應與 HTML 內容，並透過 `docker compose logs` 檢視各服務運作狀況，最後執行 `docker compose down`。
  - CI 流程驗證：向 `main` 發起 PR，觀察 GitHub Actions 完整作業之觸發、容器建置、背景啟動與 Smoke Test 探針之回傳結果。
- Open Questions:
  - 執行 Smoke Test 時，是否需要額外啟動 `loadgenerator`（`src/loadgenerator/Dockerfile:1-43`）進行數十次模擬請求，或是以輕量 HTTP GET 探針確認前端首頁渲染即可？
