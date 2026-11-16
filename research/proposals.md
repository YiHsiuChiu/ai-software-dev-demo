## Solution Proposals

### Context
- Request: 開發新功能，在結帳時訂單金額滿 US$200 即免除運費。
- Research Source: `research/free_shipping_research.md`（第 1 節 gRPC 協定定義、第 2 節 運費計算服務、第 3 節 結帳編排服務、第 4 節 前端展示與流程）。

---

### Proposal 1 — 在結帳編排層（`checkoutservice`）與前端（`frontend`）實作免運判定（不更動 Proto）
- Overview:
  本方案維持現有 gRPC Proto 介面不變，利用商品原始價格皆以 USD 定義的特性，由負責價格彙整與訂單編排的 `checkoutservice` 以及負責購物車顯示的 `frontend` 自行計算商品總額；當商品總額達到 US$200 時，將運費設定為 0。
- Key Changes:
  - `checkoutservice`：
    - 在 `src/checkoutservice/main.go:287-310`（`prepareOrderItemsAndShippingQuoteFromCart`）中，於取得品項原始 USD 價格時計算商品 USD 總額。若商品 USD 總額 $\ge$ 200，則將 `out.shippingCostLocalized` 設為 0 元（同使用者幣別），不再計入運費。
    - 在 `src/checkoutservice/money/money.go:1-132` 補充金額比對（如 `Compare` 或 `GreaterThanOrEqual`）與零元產生輔助函式。
  - `frontend`：
    - 在 `src/frontend/handlers.go:251-318`（`viewCartHandler`）中，於加總購物車商品 USD 金額時進行相同門檻判斷；若滿 US$200，將 `shippingCost` 設為 0 元傳遞至模板 `src/frontend/templates/cart.html:88-96`，確保購物車顯示之運費與最終結帳一致。
- Trade-offs:
  - 優點：
    1. 不需修改 `protos/demo.proto` 介面，完全避免多語言微服務重新產生 gRPC 程式碼（`genproto.sh`）的維護負擔與相容性風險。
    2. `shippingservice` 保持單純與無外部依賴狀態。
    3. 改動範圍收斂在 Go 服務層（`checkoutservice` 與 `frontend`）。
  - 缺點與風險：
    1. 免運門檻規則分散在 `frontend`（展示）與 `checkoutservice`（實際扣款），兩處需要維持一致的判斷邏輯。
- Validation:
  - 單元測試：在 `src/checkoutservice/money/money_test.go` 加入新增金額比較函式的測試案例；針對 `prepareOrderItemsAndShippingQuoteFromCart` 建立模擬測試，驗證小於 $200 與大於等於 $200 時的運費設定。
  - 整合驗證：透過 `docker compose up --build` 啟動，於前端購物車加入商品：
    - 加入低於 $200 商品（例如 Sunglasses $19.99），驗證 `/cart` 運費顯示為 $8.99，結帳總額包含運費。
    - 加入達 $200 商品（例如 Watch $109.99 × 2 = $219.98），驗證 `/cart` 運費為 $0.00，結帳扣款與確認頁面顯示之總額均不含運費。
- Open Questions:
  - 當使用者切換非 USD 幣別（例如 EUR、JPY）時，免運標準是固定依照商品原始 USD 小計達到 200 判定，抑或是依照換匯後的該幣別等值金額判定？（既有架構下各商品原始售價均為 USD）。

---

### Proposal 2 — 擴充 Proto 介面並在運費服務（`shippingservice`）集中實作免運判定（更動 Proto）
- Overview:
  本方案修改 gRPC 協定，在運費報價請求中新增訂單金額欄位。由呼叫端傳入當前訂單金額，運費計算與免運規則統一收斂於 `shippingservice`，達成單一邏輯來源（Single Source of Truth）。
- Key Changes:
  - `protos/demo.proto`：
    - 在 `protos/demo.proto:112-115` 的 `GetQuoteRequest` 中新增欄位：`Money items_subtotal_usd = 3;`。
    - 重新執行相關服務的 `genproto.sh`（包含 `shippingservice`、`checkoutservice`、`frontend`）。
  - `shippingservice`：
    - 修改 `src/shippingservice/main.go:117-132`（`GetQuote`），判斷 `in.ItemsSubtotalUsd`：若其金額 $\ge$ 200 USD，則回傳 `CostUsd` 為 0 元；否則呼叫 `src/shippingservice/quote.go:39-45`（`CreateQuoteFromFloat(8.99)`）回傳固定運費。
    - 修改 `src/shippingservice/shippingservice_test.go:26-56` 單元測試，補充分別大於與小於 200 USD 的報價斷言。
  - `checkoutservice` 與 `frontend`：
    - 在 `src/checkoutservice/main.go:312-321`（`quoteShipping`）與 `src/frontend/rpc.go:87-97`（`getShippingQuote`）中，呼叫 `GetQuote` 時填入已計算好的商品總額 `items_subtotal_usd`。
- Trade-offs:
  - 優點：
    1. 運費定價邏輯（一般運費 $8.99、滿額免運 $0）完全收斂在 `shippingservice` 領域服務內，呼叫端（`frontend` 與 `checkoutservice`）僅單純查詢運費，無須了解促銷或免運業務規則。
  - 缺點與風險：
    1. 需要更動 `protos/demo.proto` 並重新生成多個服務的 gRPC Client/Server stub，若有其他未更新的調用方可能會造成欄位缺漏或介面不一致。
    2. 涉及修改的服務數量較多（跨 `protos`、`shippingservice`、`checkoutservice`、`frontend`），異動影響面較大。
- Validation:
  - 單元測試：在 `src/shippingservice/shippingservice_test.go` 執行 `go test ./...`，驗證滿額回傳 0 元與未滿額回傳 8.99 元。
  - 整合驗證：透過 `docker compose build shippingservice checkoutservice frontend` 重建容器，驗證購物車頁面與結帳流程在滿額與未滿額時均能取得正確運費。
- Open Questions:
  - 是否允許修改既有的 Protocol Buffers 定義（`protos/demo.proto`）以及重新執行 proto codegen？
