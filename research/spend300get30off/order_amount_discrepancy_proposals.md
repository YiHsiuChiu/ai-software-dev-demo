## Solution Proposals

### Context
- Request: 解決購買 3 支手錶時，購物車預估金額為 299.97 元，但結帳完成頁面顯示金額為 329.97 元（未扣除滿 300 折 30）之金額不一致問題。
- Research Source: `research/spend300get30off/order_amount_discrepancy_research.md`（第 1 節 前端購物車金額計算與展示實作、第 2 節 結帳編排服務實作、第 3 節 gRPC 協定定義與 Protobuf 描述子現況、第 4 節 前端訂單結帳與完成頁面實作、Data Flow 章節）。

---

### Proposal 1 — 前端直接根據訂單品項與門檻計算折扣並展示（前端獨立計算方案 / Frontend Autonomous Calculation）

- Overview:
  本方案不依賴 gRPC 傳輸未完整編譯進 protobuf descriptor 的欄位，延續前端既有自行計算 `totalPaid` 的架構模式。在 `frontend` 的 `placeOrderHandler` 中，於累加各品項金額時一併比對商品 USD 小計是否達 300 USD 門檻；若達門檻，直接計算 30 USD 折扣並自 `totalPaid` 扣除，並將折扣物件注入 `order.html` 模板，確保完成頁面與購物車頁面金額完全一致。

- Key Changes:
  - `src/frontend/handlers.go:406-430`（`placeOrderHandler`）：
    - 走訪 `order.GetOrder().GetItems()` 時，同時累加原始商品 USD 金額 `itemsUSD`。
    - 比對 `itemsUSD` 是否 $\ge$ 300 USD；若滿額，透過 `fe.convertCurrency` 將 30 USD 轉換為目前幣別 `discountCost`。
    - 從 `totalPaid` 中扣減 `discountCost`：`totalPaid = money.Must(money.Sub(totalPaid, *discountCost))`。
    - 於模板資料中加入 `"discount": discountCost`。
  - `src/frontend/templates/order.html:56-65`：
    - 將折扣顯示條件調整為讀取傳入之 `.discount`（或相容 `or .discount .order.Discount`），確保完成頁面能正確顯示 `- $30.00`。

- Trade-offs:
  - 優點：
    1. **完全不依賴 Protoc 編譯器或描述子更新**：徹底避開因環境無 `protoc` 工具鏈導致 gRPC 反序列化欄位為 `nil` 的問題，穩定可靠。
    2. **改動範圍極小且快速**：僅需修改 `src/frontend/handlers.go` 與 `src/frontend/templates/order.html` 兩個檔案，改動十餘行程式碼即可立即修正。
    3. **符合前端既有模式**：在現有實作中，`totalPaid` 本來就是由前端在收到回應後自行走訪 `order.GetOrder().GetItems()` 累加計算，而非直接自後端取值。
  - 缺點與風險：
    1. **促銷判定邏輯重複**：滿 300 折 30 的門檻判定同時存在於前端與後端，若未來調整門檻需同步修改兩處。

- Validation:
  - 整合驗證：
    - 重新建置並啟動前端容器：`docker compose up --build -d frontend`。
    - 加入 3 支手錶（$329.97），檢視購物車顯示 `Total: $299.97`。
    - 點擊送出訂單，確認訂單完成頁 `/order` 正確顯示 `Discount: - $30.00` 以及 `Total Paid: $299.97`。
    - 加入未滿 $300 商品（例如 1 支手錶 $109.99），確認 `/order` 正常顯示 `$109.99` 且無折扣行。

- Open Questions:
  - 無。

---

### Proposal 2 — 使用 Docker 容器內的 Protoc 重新生成完整的 Protobuf 描述子（容器化 Protobuf Codegen 方案 / Containerized Protobuf Codegen）

- Overview:
  本方案透過具備 `protoc` 與 `protoc-gen-go` 工具鏈的 Docker 容器，重新執行 `genproto.sh` 編譯 `protos/demo.proto`，產出包含完整二進位描述子（`file_demo_proto_rawDesc`）與相依索引（`file_demo_proto_depIdxs`）的 `demo.pb.go`，使 `OrderResult.discount` 能正常透過 gRPC 二進位協定傳輸至前端。

- Key Changes:
  - `src/checkoutservice/genproto/demo.pb.go:1831-2210` 與 `src/frontend/genproto/demo.pb.go:1831-2210`：
    - 啟動包含 protoc 工具的容器執行代碼生成，更新 `file_demo_proto_rawDesc` 與 `file_demo_proto_depIdxs`，註冊欄位 6 `discount` 之型態定義。
  - `src/shippingservice/genproto/demo.pb.go`：
    - 同步更新以保持各服務 protobuf stub 一致。
  - 前端與後端現有業務程式碼（`src/checkoutservice/main.go`、`src/frontend/handlers.go` 與 `src/frontend/templates/order.html`）：
    - **維持既有邏輯不變**，依賴已實作之 `order.GetOrder().GetDiscount()` 即可自動取得折扣物件。

- Trade-offs:
  - 優點：
    1. **協定契約完整乾淨**：`OrderResult.Discount` 能在 gRPC 傳輸層真實傳遞，完全符合 Contract-Driven 架構設計理念。
    2. **前端無需重複實作折扣判定**：前端 `placeOrderHandler` 既有的 `if d := order.GetOrder().GetDiscount()` 判斷可直接生效，無須在前端二次計算。
  - 缺點與風險：
    1. **工具鏈相依性高**：因本機無安裝 `protoc`，需依賴 Docker 容器環境進行臨時編譯，若容器環境設定不當可能造成建置阻礙。
    2. **產生大量程式碼 Diff**：`protoc` 重新產生會更動整個 protobuf runtime 樣板，引入數百行至上千行的自動生成程式碼變更。

- Validation:
  - 單元驗證：於容器內重新執行 Go protobuf codegen 後，執行 Go 編譯檢查確認無語法錯誤。
  - 整合驗證：執行 `docker compose up --build -d`，重啟 `checkoutservice` 與 `frontend`，下單 3 支手錶，確認完成頁讀取到的 `order.GetDiscount()` 非空，`Total Paid` 正確顯示 $299.97。

- Open Questions:
  - 本地 Docker 環境是否支援掛載目錄並執行具備 `protoc` 與 `protoc-gen-go` 的客製容器？
