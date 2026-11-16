# Free Shipping on Orders Over US$200 — Design Document

## 1. Current Context

- Online Boutique 為多語言微服務架構，其中購物車檢視與訂單結帳流程涉及 `frontend`、`checkoutservice` 與 `shippingservice`。
- 目前 `shippingservice` 的 gRPC 介面 `GetQuote` 僅接收 `Address` 與 `CartItem[]`，其實作完全忽略輸入參數，固定回傳美金 8 元 99 分（USD $8.99）。
- `frontend`（於購物車頁面 `/cart`）與 `checkoutservice`（於結帳流程 `/cart/checkout`）各自獨立呼叫 `shippingservice.GetQuote`，再透過 `currencyservice` 轉匯為使用者選定幣別後加總至總金額。
- 現有系統缺乏訂單金額門檻判斷，即使購物車金額極高，使用者仍需支付固定運費。本次需求旨在達成：當訂單商品金額達 US$200 時免除運費（即運費為 $0.00）。

---

## 2. Requirements

### Functional Requirements

- **滿額免運判定**：當購物車或訂單之商品總額（以 USD 計算）$\ge$ US$200 時，`shippingservice.GetQuote` 應回傳運費 USD $0.00。
- **未滿額維持既有運費**：當商品總額 $<$ US$200（或未提供金額）時，`shippingservice.GetQuote` 維持既有行為，回傳固定運費 USD $8.99。
- **展示與結帳一致性**：
  - 前端購物車頁面（`/cart`）中顯示的運費（Shipping）與總金額（Total）須反映免運結果。
  - 結帳扣款（`paymentservice.Charge`）與訂單確認頁面（`/cart/checkout` 及確認信）須收取且顯示正確的免運後總額。
- **多幣別相容**：無論使用者選用何種幣別瀏覽或結帳，免運標準均以商品原始定價之 USD 總額為基準，0 元運費轉換後在任何目標幣別中均為 0。

### Non-Functional Requirements

- **介面向後相容性**：在 `GetQuoteRequest` 擴充的欄位為可選（optional），若未帶入該欄位則降級為預設運費（USD $8.99），不破壞既有呼叫。
- **改動範圍限制**：僅更動本次需求直接相關的 proto 定義與 Go 服務，不影響其他無關微服務（如 `paymentservice`、`emailservice`、`adservice` 等）。

---

## 3. Design Decisions

### Selected Approach

- **選定方案：** Proposal 2 — 擴充 Proto 介面並在運費服務（`shippingservice`）集中實作免運判定。
- **核心設計：**
  1. 在 `protos/demo.proto` 的 `GetQuoteRequest` 擴充 `Money items_subtotal_usd = 3;` 欄位。
  2. `shippingservice` 的 `GetQuote` 根據傳入的 `ItemsSubtotalUsd` 進行門檻判定：若 $\ge$ US$200 則回傳 USD 0 運費，否則回傳 USD 8.99。
  3. `checkoutservice` 在準備訂單時，先向 `productcatalogservice` 取得商品價格並累加 USD 商品總額，在呼叫 `quoteShipping` 時帶入 `ItemsSubtotalUsd`。
  4. `frontend` 在渲染購物車前，計算購物車商品的 USD 總額，在呼叫 `getShippingQuote` 時帶入 `ItemsSubtotalUsd`。
- **選擇原因：**
  運費計算與促銷免運規則統一收斂在領域服務 `shippingservice` 內部（Single Source of Truth），`frontend` 與 `checkoutservice` 作為調用方僅需傳入商品小計並消費報價，無須各自維護重複的免運業務邏輯。
- **Trade-offs：**
  - *優點*：職責分工清晰，運費業務規則集中管理，未來若運費門檻或基礎運費費率調整，僅需修改 `shippingservice`。
  - *代價與限制*：需要更新 `protos/demo.proto` 並同步更新受影響服務的 proto 生成程式碼（`genproto`）。

---

## 4. Technical Design

### Core Components

1. **`protos/demo.proto`**
   - 擴充 `GetQuoteRequest`，新增 `Money items_subtotal_usd = 3;`。
2. **`shippingservice`**
   - 更新 `genproto` 中的 Go protobuf 定義。
   - 在 `main.go` 的 `GetQuote` 檢查傳入的 `ItemsSubtotalUsd`：若 `Units > 200 || (Units == 200 && Nanos >= 0)`，則回傳 `CostUsd` 為 0 USD（`Units: 0, Nanos: 0`）；否則回傳預設運費 USD 8.99。
   - 更新 `shippingservice_test.go` 單元測試覆蓋滿額與未滿額情境。
3. **`checkoutservice`**
   - 更新 `genproto` 中的 Go protobuf 定義。
   - 在 `prepareOrderItemsAndShippingQuoteFromCart` 累加購物車商品的原始 USD 總額（`itemsSubtotalUSD`）。
   - 修改 `quoteShipping`，將 `itemsSubtotalUSD` 填入 `GetQuoteRequest.ItemsSubtotalUsd`。
4. **`frontend`**
   - 更新 `genproto` 中的 Go protobuf 定義。
   - 在 `viewCartHandler` 中，巡訪購物車品項取得各商品之 `p.GetPriceUsd()` 並累加 USD 商品總額，調整呼叫順序將 USD 總額傳入 `getShippingQuote`。
   - 修改 `getShippingQuote` 函式簽名，將傳入的 USD 總額賦值給 `GetQuoteRequest.ItemsSubtotalUsd`。

### Data Flow & Integration

```
[Browser / User]
       │
       ▼
[frontend: viewCartHandler / placeOrderHandler]
       │
       ├─► 查詢各品項 Product.GetPriceUsd() 並計算商品 USD 總額 (itemsSubtotalUSD)
       │
       ├─► [shippingservice: GetQuote]
       │       Request:  { Address, Items, ItemsSubtotalUsd: itemsSubtotalUSD }
       │       Response: itemsSubtotalUSD >= $200 ? $0.00 USD : $8.99 USD
       │
       ├─► [currencyservice: Convert]
       │       將運費 ($0 或 $8.99) 轉為使用者幣別 (0 USD 轉換後為 0)
       │
       ▼
計算訂單總額 (Items Total + Shipping Cost)，呈現於購物車或執行扣款
```

### Files Changed

- `protos/demo.proto:112-116` — 在 `GetQuoteRequest` 訊息結構中新增 `Money items_subtotal_usd = 3;`。
- `src/shippingservice/genproto/demo.pb.go:713-725` — 更新 Go protobuf 生成結構，加入 `ItemsSubtotalUsd *Money` 欄位與對應 getter。
- `src/shippingservice/main.go:116-132` — 在 `GetQuote` 中新增滿額 US$200 判定邏輯，滿額時回傳 0 USD 運費。
- `src/shippingservice/shippingservice_test.go:26-56` — 更新 `TestGetQuote` 單元測試，加入滿額免運（$\ge \$200$）與未滿額固定運費（$<\$200$）測試案例。
- `src/checkoutservice/genproto/demo.pb.go:713-725` — 更新 Go protobuf 生成結構，加入 `ItemsSubtotalUsd *Money` 欄位與對應 getter。
- `src/checkoutservice/main.go:287-321` — 在 `prepareOrderItemsAndShippingQuoteFromCart` 計算商品 USD 總額並調整 `quoteShipping` 帶入該總額。
- `src/frontend/genproto/demo.pb.go:713-725` — 更新 Go protobuf 生成結構，加入 `ItemsSubtotalUsd *Money` 欄位與對應 getter。
- `src/frontend/rpc.go:87-97` — 修改 `getShippingQuote` 函式參數以接收 `subtotalUsd *pb.Money` 並設定至 `GetQuoteRequest`。
- `src/frontend/handlers.go:270-305` — 在 `viewCartHandler` 中調整順序，於計算商品價格時累加 USD 總額並傳入 `fe.getShippingQuote`。

---

## 5. Implementation Plan

### Step 1 — 擴充 gRPC 協定定義與更新相關微服務的 Stub
- **修改檔案**：
  - `protos/demo.proto:112-116`
  - `src/shippingservice/genproto/demo.pb.go:713-725`
  - `src/checkoutservice/genproto/demo.pb.go:713-725`
  - `src/frontend/genproto/demo.pb.go:713-725`
- **具體修改內容**：
  - 在 `GetQuoteRequest` 中加入 `Money items_subtotal_usd = 3;`。
  - 同步更新 Go 服務所使用的 generated protobuf 程式碼，確保具備 `ItemsSubtotalUsd` 欄位與 `GetItemsSubtotalUsd()` 方法。
- **完成條件**：
  - 各服務的程式碼能順利引用 `req.ItemsSubtotalUsd` 與 `req.GetItemsSubtotalUsd()` 且編譯無語法錯誤。

### Step 2 — 於 `shippingservice` 實作免運門檻判斷與單元測試
- **修改檔案**：
  - `src/shippingservice/main.go:116-132`
  - `src/shippingservice/shippingservice_test.go:26-56`
- **具體修改內容**：
  - 在 `GetQuote` 方法中讀取 `in.GetItemsSubtotalUsd()`。
  - 若 `subtotal != nil` 且其金額大於或等於 200 USD（即 `Units > 200 || (Units == 200 && Nanos >= 0)`），回傳 `CostUsd` 為 0 USD（`Units: 0, Nanos: 0`）。
  - 若未達 200 USD 或未提供金額，維持回傳固定運費 USD 8.99。
  - 在 `shippingservice_test.go` 中擴充測試，分別驗證 subtotal 為 $199.99（預期 $8.99）、$200.00（預期 $0.00）、$250.00（預期 $0.00）及未傳入金額時的回傳值。
- **完成條件**：
  - `shippingservice_test.go` 單元測試全數通過。

### Step 3 — 於 `checkoutservice` 串接商品金額計算與運費查詢
- **修改檔案**：
  - `src/checkoutservice/main.go:287-321`
- **具體修改內容**：
  - 修改 `quoteShipping(ctx context.Context, address *pb.Address, items []*pb.CartItem, itemsSubtotalUSD *pb.Money)`，將 `itemsSubtotalUSD` 設定至 `GetQuoteRequest.ItemsSubtotalUsd`。
  - 在 `prepareOrderItemsAndShippingQuoteFromCart` 中，在取得各品項資訊（`prepOrderItems`）或巡訪購物車時加總各品項 USD 價格（`product.GetPriceUsd()` 乘以品項數量），得到 `itemsSubtotalUSD`，並傳遞給 `quoteShipping`。
- **完成條件**：
  - `checkoutservice` 在結帳時正確向 `shippingservice` 傳送 USD 商品總額，並依據回應正確扣款。

### Step 4 — 於 `frontend` 購物車檢視串接商品總額計算與運費查詢
- **修改檔案**：
  - `src/frontend/rpc.go:87-97`
  - `src/frontend/handlers.go:270-305`
- **具體修改內容**：
  - 修改 `src/frontend/rpc.go` 中的 `getShippingQuote`，接受 `subtotalUsd *pb.Money` 參數並填入 `GetQuoteRequest`。
  - 修改 `src/frontend/handlers.go` 的 `viewCartHandler`：在迴圈遍歷購物車商品取得 `p.GetPriceUsd()` 時，除了轉換目標貨幣外，亦累加 USD 商品總額（`itemsSubtotalUSD`）。
  - 將 `fe.getShippingQuote` 呼叫移至計算完 USD 商品總額之後，將 `itemsSubtotalUSD` 作為參數傳入。
- **完成條件**：
  - 瀏覽 `/cart` 時，當購物車商品滿 US$200，畫面顯示之運費為 $0.00，總金額正確扣減運費。

---

## 6. Testing Strategy

### Unit Tests

- **`shippingservice` (`src/shippingservice/shippingservice_test.go`)**：
  1. *未滿額測試*：傳入 `items_subtotal_usd` 為 $199.99，驗證回傳 `CostUsd` 為 8 USD、990000000 Nanos（$8.99）。
  2. *滿額邊界測試*：傳入 `items_subtotal_usd` 為 $200.00（`Units: 200, Nanos: 0`），驗證回傳 `CostUsd` 為 0 USD、0 Nanos（$0.00）。
  3. *超額測試*：傳入 `items_subtotal_usd` 為 $350.00，驗證回傳 `CostUsd` 為 0 USD、0 Nanos（$0.00）。
  4. *向下相容測試*：傳入 `items_subtotal_usd` 為 `nil`，驗證回傳 `CostUsd` 維持固定運費 $8.99。

### Integration / Functional Tests

- **Docker Compose 環境端到端驗證**：
  - 重建並重啟服務：`docker compose up --build -d shippingservice checkoutservice frontend`。
  - **情境 1（未滿額購物車）**：
    - 新增 1 件 "Sunglasses"（單價 $19.99）。
    - 進入 `/cart`，確認運費欄位顯示 `$8.99`，總額顯示 `$28.98`。
    - 進行結帳，確認訂單成功建立，總扣款金額為 `$28.98`。
  - **情境 2（滿額免運購物車）**：
    - 新增 2 件 "Watch"（單價 $109.99，小計 $219.98 $\ge \$200$）。
    - 進入 `/cart`，確認運費欄位顯示 `$0.00`，總額顯示 `$219.98`。
    - 進行結帳，確認訂單成功建立，確認信與訂單完成頁面之總扣款金額為 `$219.98`（無運費）。
  - **情境 3（多幣別換匯免運）**：
    - 切換幣別為 EUR，選購總價超過 200 USD 的商品組合。
    - 確認運費仍為 0 EUR，商品總計正確換匯。

---

## 7. References

- Research Document: [`research/free_shipping_research.md`](file:///c:/Users/NCYU/Desktop/陳允瀚/軟工實務助教/115學年/教學計劃/ai-software-dev-demo/research/free_shipping_research.md)
- Solution Proposals: [`research/proposals.md`](file:///c:/Users/NCYU/Desktop/陳允瀚/軟工實務助教/115學年/教學計劃/ai-software-dev-demo/research/proposals.md) (Proposal 2 — 擴充 Proto 介面並在運費服務集中實作免運判定)
- Design Document Template: [`prompts/design_doc_template.md`](file:///c:/Users/NCYU/Desktop/陳允瀚/軟工實務助教/115學年/教學計劃/ai-software-dev-demo/prompts/design_doc_template.md)
