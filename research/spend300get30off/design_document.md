# [FEATURE] Design Document: 訂單滿 300 折 30 與免運先行折抵 (Spend $300 Get $30 Off)

## 1. Current Context

目前 Online Boutique 系統由多個微服務協同處理購物車與結帳流程：
- `frontend` 提供購物車檢視（`GET /cart`）與訂單送出（`POST /cart/checkout`）介面。
- `checkoutservice` 負責編排結帳業務流程，向 `cartservice` 取得購物車項目，向 `productcatalogservice` 查詢品項 USD 原價，透過 `currencyservice` 進行貨幣換匯，並向 `shippingservice` 取得運費報價。
- `shippingservice.GetQuote` 目前依據請求中的 `items_subtotal_usd` 判定是否達到 200 USD：若達門檻則回傳 0 USD（免運費），否則回傳固定運費 8.99 USD。
- `paymentservice` 接收 `checkoutservice` 計算之訂單總金額進行信用卡模擬扣款。

現有系統中完全不存在任何滿額折抵或折扣機制，商品小計與運費均以原價加總計算。本次功能需求旨在實作「訂單金額滿 300 折 30」功能，並在結帳與購物車頁面明確標明折扣金額；特別要求此折扣必須在評估滿 200 免運費門檻之前先予以扣除。

---

## 2. Requirements

### Functional Requirements

1. **滿額折扣判定與扣減（滿 300 折 30）**：
   - 當訂單商品的美金小計（Items Subtotal USD）大於等於 300 USD 時，系統自動提供 30 USD 的折扣。
   - 若商品美金小計未滿 300 USD，則折扣為 0。
2. **折扣在免運費之前計算（Pre-Shipping Discount Evaluation）**：
   - 運費報價與免運費門檻（滿 200 USD 免運）判定，必須以**折抵後之美金小計**（`discounted_subtotal_usd = subtotal_usd - discount_usd`）作為輸入傳遞給 `shippingservice.GetQuote`。
   - 若折抵後之美金小計 $\ge$ 200 USD，享有免運費（0 USD）；若折抵後未達 200 USD，則收取標準運費（8.99 USD）。
3. **明確標明折扣金額（Display Discount Amount）**：
   - 在購物車摘要頁面（`/cart`）中，若符合折扣條件，明確顯示折扣明細列（如 `Discount (Orders $300+): - $30.00`），並以折抵後金額呈現訂單總額（Total）。
   - 在訂單完成確認頁面（`/order`）中，顯示折扣金額與扣抵後實際支付總額（Total Paid）。
4. **結帳扣款金額一致性**：
   - 送交 `paymentservice.Charge` 的扣款總金額，必須精準扣除折扣金額：`total = itemsTotalLocalized - discountLocalized + shippingCostLocalized`。
5. **多幣別轉換支援**：
   - 使用者切換非 USD 幣別時，30 USD 的折扣金額透過 `currencyservice.Convert` 轉換為使用者選定幣別進行扣減與展示。

### Non-Functional Requirements

- **API 契約清晰與向後相容**：在 `protos/demo.proto` 的 `OrderResult` 擴充 `Money discount = 6;` 欄位，不破壞既有欄位編號與型態。
- **改動範圍限制**：聚焦於結帳編排（`checkoutservice`）、前端（`frontend`）與 Proto 定義，不引入非必要的套件或對其他微服務進行不相關之變更。
- **數值精準性**：金額加減與比較遵循 `pb.Money` 結構規範（`units` 與 `nanos`），避免浮點數運算誤差。

---

## 3. Design Decisions

### Selected Approach

- **選定方案：** Proposal 1 — 擴充 Proto 契約並集中由結帳服務主導折扣（Contract-Driven Proposal）。
- **核心設計：**
  1. 在 `protos/demo.proto` 的 `OrderResult` 訊息新增 `Money discount = 6;` 欄位，並重新產生 Go gRPC 程式碼。
  2. 在 `src/checkoutservice/money/` 與 `src/frontend/money/` 中補充 `Sub`（減法）與 `Compare`（比對）公用運算函式。
  3. `checkoutservice` 於準備訂單（`prepareOrderItemsAndShippingQuoteFromCart`）時計算商品 USD 小計：若小計 $\ge$ 300 USD，產生 30 USD 折扣；以**折抵後小計**傳遞給 `shippingservice.GetQuote` 取得免運運費。
  4. `checkoutservice` 換匯折扣金額後計算正確扣款總額 `total`，扣款後將 `discount` 封裝於 `OrderResult` 回傳。
  5. `frontend` 在購物車頁面進行相同門檻換算並將折後小計傳給運費查詢，於購物車模板與訂單完成模板中新增 Discount 顯示列。
- **選擇原因：**
  - **Single Source of Truth**：由 `checkoutservice` 作為交易核心計算並在 `OrderResult` 中正式記錄折扣，前端與後續服務直接消費標準契約資料，消除前後端分散推算導致金額不一致之風險。
  - **資料完整性**：訂單交易實體完整保留折扣歷史紀錄，符合企業微服務架構之最佳實踐。
- **Trade-offs：**
  - *優點*：介面語意完整，前後端資料流清晰一致，擴充性佳。
  - *缺點*：需修改 `demo.proto` 並重新編譯 `checkoutservice` 與 `frontend` 的 `demo.pb.go`。

---

## 4. Technical Design

### Core Components

1. **`protos/demo.proto`**：
   - 擴充 `OrderResult` 定義，新增 `Money discount = 6;`。
2. **`src/checkoutservice/money/` & `src/frontend/money/`**：
   - 新增 `Sub(l, r pb.Money) (pb.Money, error)`：利用既有 `Sum(l, Negate(r))` 達成安全金額扣減。
   - 新增 `Compare(l, r pb.Money) (int, error)`：比較同幣別金額大小（回傳 1, 0, -1）。
3. **`src/checkoutservice/`**：
   - 擴充 `orderPrep` 結構，增加 `discountLocalized *pb.Money`。
   - 修改 `prepareOrderItemsAndShippingQuoteFromCart`：判定美金小計滿 300 USD 時折抵 30 USD；以折後美金小計傳給 `quoteShipping`。
   - 修改 `PlaceOrder`：扣款總金額扣除 `discountLocalized`，並填入 `OrderResult.Discount`。
4. **`src/frontend/`**：
   - 修改 `viewCartHandler`：判定滿 300 USD 折 30 USD，以折後美金小計傳給 `getShippingQuote`，並將 `discount` 注入模板。
   - 修改 `placeOrderHandler`：自 `order.GetDiscount()` 讀取折扣金額，精確計算實付總額 `totalPaid`。
   - 修改 `cart.html` 與 `order.html`：新增 Discount 列顯示折扣金額（如 `- $30.00`）。

### Data Flow & Integration

```
[Browser]
    │  1. GET /cart
    ▼
[frontend: viewCartHandler]
    │  - 計算 itemsUSD (商品 USD 原價總額)
    │  - 若 itemsUSD >= 300 USD:
    │      discountUSD = 30 USD
    │      discountedUSD = itemsUSD - 30 USD
    │      discountLocalized = convertCurrency(discountUSD, currentCurrency)
    │  - 呼叫 shippingservice.GetQuote(&pb.GetQuoteRequest{ItemsSubtotalUsd: discountedUSD})
    │      └─► shippingservice 判定 discountedUSD 是否 >= 200 USD (決定運費 0 或 8.99 USD)
    │  - totalPrice = itemsTotalLocalized - discountLocalized + shippingCost
    ▼
[cart.html] 渲染 Items, Shipping, Discount (- $30.00), Total

-----------------------------------------------------------------------------------------

[Browser]
    │  2. POST /cart/checkout
    ▼
[frontend: placeOrderHandler]
    │  - 呼叫 checkoutservice.PlaceOrder(req)
    ▼
[checkoutservice: PlaceOrder]
    │  - 呼叫 prepOrderItems 計算 itemsSubtotalUSD
    │  - 若 itemsSubtotalUSD >= 300 USD:
    │      discountUSD = 30 USD
    │      discountedUSD = itemsSubtotalUSD - 30 USD
    │      discountLocalized = convertCurrency(discountUSD, userCurrency)
    │  - 呼叫 quoteShipping(&discountedUSD)
    │      └─► shippingservice 判定 discountedUSD 是否 >= 200 USD (決定運費 0 或 8.99 USD)
    │  - total = itemsTotalLocalized - discountLocalized + shippingCostLocalized
    │  - paymentservice.Charge(total, credit_card) (扣除折抵後金額)
    │  - 回傳 OrderResult{ OrderId, ShippingCost, Items, Discount: discountLocalized }
    ▼
[frontend: placeOrderHandler]
    │  - totalPaid = shippingCost + sum(itemCost * qty) - order.Discount
    ▼
[order.html] 渲染 Order ID, Tracking #, Discount (- $30.00), Total Paid
```

### Files Changed

- `protos/demo.proto:209-216` — 在 `OrderResult` 訊息末尾新增 `Money discount = 6;`
- `src/checkoutservice/genproto/demo.pb.go:1-600` — 重新編譯產生的 gRPC protobuf Go 程式碼（包含 `Discount` 欄位與 getter）
- `src/frontend/genproto/demo.pb.go:1-600` — 重新編譯產生的 gRPC protobuf Go 程式碼（包含 `Discount` 欄位與 getter）
- `src/checkoutservice/money/money.go:93-132` — 新增 `Sub` 與 `Compare` 金額運算函式
- `src/checkoutservice/money/money_test.go:227-260` — 新增 `Sub` 與 `Compare` 之單元測試
- `src/frontend/money/money.go:93-132` — 新增 `Sub` 與 `Compare` 金額運算函式
- `src/frontend/money/money_test.go:227-260` — 新增 `Sub` 與 `Compare` 之單元測試
- `src/checkoutservice/main.go:242-255` — 結帳總金額扣減折扣金額，並將 `total` 傳給 `chargeCard`
- `src/checkoutservice/main.go:264-270` — 將 `prep.discountLocalized` 賦值至 `OrderResult.Discount`
- `src/checkoutservice/main.go:281-285` — 在 `orderPrep` 結構新增 `discountLocalized *pb.Money`
- `src/checkoutservice/main.go:287-310` — 實作滿 300 折 30 邏輯，並將折抵後美金小計傳入 `quoteShipping`
- `src/frontend/handlers.go:276-324` — `viewCartHandler` 中實作滿 300 折 30，將折後金額傳入 `getShippingQuote` 並將 `discount` 注入模板
- `src/frontend/handlers.go:386-407` — `placeOrderHandler` 中依 `order.GetOrder().GetDiscount()` 扣減計算 `totalPaid`
- `src/frontend/templates/cart.html:88-96` — 新增 Discount 行顯示折扣金額
- `src/frontend/templates/order.html:56-63` — 新增 Discount 行顯示折扣金額

---

## 5. Implementation Plan

### Step 1 — 更新 Protocol Buffers 契約與產生程式碼
- **修改檔案**：
  - `protos/demo.proto:209-216`
  - `src/checkoutservice/genproto/demo.pb.go`
  - `src/frontend/genproto/demo.pb.go`
- **具體修改內容**：
  - 在 `protos/demo.proto` 的 `OrderResult` 訊息新增 `Money discount = 6;`。
  - 在 `src/checkoutservice/` 與 `src/frontend/` 執行 `genproto.sh`（或 `protoc`）重新產生 `genproto/demo.pb.go`。
- **完成條件**：
  - `demo.pb.go` 中包含 `Discount *Money` 欄位與 `GetDiscount() *Money` 方法，且專案可通過語法編譯。

### Step 2 — 擴充 Money 運算模組與單元測試
- **修改檔案**：
  - `src/checkoutservice/money/money.go:93-132`
  - `src/checkoutservice/money/money_test.go:227-260`
  - `src/frontend/money/money.go:93-132`
  - `src/frontend/money/money_test.go:227-260`
- **具體修改內容**：
  - 實作 `Sub(l, r pb.Money) (pb.Money, error)` 與 `Compare(l, r pb.Money) (int, error)`。
  - 撰寫單元測試覆蓋正常扣減、不同幣別錯誤、大小比對正負等場景。
- **完成條件**：
  - `cd src/checkoutservice/money && go test -v ./...` 與 `cd src/frontend/money && go test -v ./...` 全數測試通過。

### Step 3 — 在結帳服務實作滿額折扣與折後免運詢價
- **修改檔案**：
  - `src/checkoutservice/main.go:242-255`
  - `src/checkoutservice/main.go:264-270`
  - `src/checkoutservice/main.go:281-285`
  - `src/checkoutservice/main.go:287-310`
- **具體修改內容**：
  - 在 `orderPrep` 結構新增 `discountLocalized *pb.Money`。
  - 在 `prepareOrderItemsAndShippingQuoteFromCart` 中，檢查 `itemsSubtotalUSD` 是否 $\ge$ 300 USD。若成立，計算 30 USD 折扣，並以 `itemsSubtotalUSD - 30 USD` 作為 `ItemsSubtotalUsd` 呼叫 `quoteShipping`。
  - 將 30 USD 換匯為 `userCurrency`，並於 `PlaceOrder` 中從總額扣除折扣：`total = itemsTotalLocalized - discountLocalized + shippingCostLocalized`。
  - 於 `OrderResult` 填入 `Discount: prep.discountLocalized`。
- **完成條件**：
  - `checkoutservice` 正常編譯，滿 300 扣款金額正確扣除 30 元，傳入運費服務的小計為折後金額。

### Step 4 — 在前端服務與模板實作折扣顯示與折後免運詢價
- **修改檔案**：
  - `src/frontend/handlers.go:276-324`
  - `src/frontend/handlers.go:386-407`
  - `src/frontend/templates/cart.html:88-96`
  - `src/frontend/templates/order.html:56-63`
- **具體修改內容**：
  - 在 `viewCartHandler` 中，若 `itemsUSD` $\ge$ 300 USD，產生 30 USD 之本地貨幣折扣，將折後美金小計傳給 `getShippingQuote`，並向模板注入 `discount`。
  - 在 `cart.html` 中新增 Discount 列，當有折扣時以綠色文字標明 `- {{ renderMoney .discount }}`。
  - 在 `placeOrderHandler` 中自 `order.GetOrder().GetDiscount()` 讀取折扣並自總額扣除，計算正確之 `totalPaid`。
  - 在 `order.html` 中新增 Discount 列，標明訂單享有之折扣金額。
- **完成條件**：
  - 購物車頁面與結帳成功頁面均能正確顯示折扣金額與對應之總額。

### Step 5 — 整合驗證與全系統回歸
- **修改檔案**：無（執行測試與驗證）
- **具體修改內容**：
  - 執行 `go test ./...` 驗證所有 Go 服務測試。
  - 執行 `docker compose up --build -d` 啟動所有容器。
  - 驗證以下情境：
    1. 商品金額 < 200 USD（如 150 USD）：無折扣，運費 8.99 USD。
    2. 商品金額介於 200 與 299.99 USD（如 250 USD）：無折扣，運費 0 USD（免運）。
    3. 商品金額 $\ge$ 300 USD（如 320 USD）：享有 30 USD 折扣，折後 290 USD $\ge$ 200 USD，運費 0 USD（免運），總額為 290 USD。
    4. 折扣邊界案例驗證（折後小計是否正確作為免運判定基準）。
- **完成條件**：
  - 畫面顯示、日誌請款金額與預期數值完全一致。

---

## 6. Testing Strategy

### Unit Tests

1. **金額運算單元測試 (`src/checkoutservice/money/money_test.go` & `src/frontend/money/money_test.go`)**：
   - `TestSub`：
     - 正數相減：`$100.00 - $30.00 = $70.00`
     - 包含微單位 nanos 相減：`$100.50 - $30.75 = $69.75`
     - 減至 0 元：`$30.00 - $30.00 = $0.00`
     - 幣別不符：USD 減 EUR 應回傳 `ErrMismatchingCurrency`
   - `TestCompare`：
     - 大於：`$300.00` vs `$299.99` 回傳 1
     - 等於：`$300.00` vs `$300.00` 回傳 0
     - 小於：`$299.99` vs `$300.00` 回傳 -1

### Integration / Functional Tests

透過本機 Docker Compose 環境（`docker compose up --build`）驗證完整下單購物流程：

1. **情境 A：購物車未滿 300 USD 且未滿 200 USD**
   - 加入商品總值約 $100。
   - 預期：購物車無 Discount 列，Shipping 顯示 $8.99，Total 為 $108.99。結帳成功頁 Total Paid 為 $108.99。
2. **情境 B：購物車未滿 300 USD 但滿 200 USD（既有免運行為）**
   - 加入商品總值約 $250。
   - 預期：購物車無 Discount 列，Shipping 顯示 $0.00，Total 為 $250.00。結帳成功頁 Total Paid 為 $250.00。
3. **情境 C：購物車滿 300 USD（本次新功能）**
   - 加入商品總值達 $350。
   - 預期：
     - 購物車顯示 Discount `- $30.00`。
     - 依「折扣在免運前計算」規則，折後商品總額為 $320 $\ge$ $200，故 Shipping 顯示 `$0.00`。
     - Total 顯示 `$320.00`。
     - 結帳送出後，`paymentservice` 容器日誌顯示扣款金額為 `$320.00`。
     - 訂單完成頁顯示 Discount `- $30.00`，Total Paid 為 `$320.00`。
4. **情境 D：非 USD 幣別結帳**
   - 切換貨幣至 EUR，加入商品達等值 300 USD 以上。
   - 預期：Discount 列顯示換匯後之 EUR 折扣，總額正確扣抵。

---

## 7. References

- [research.md](file:///c:/Users/NCYU/Desktop/陳允瀚/軟工實務助教/115學年/教學計劃/ai-software-dev-demo/research/spend300get30off/research.md)
- [proposals.md](file:///c:/Users/NCYU/Desktop/陳允瀚/軟工實務助教/115學年/教學計劃/ai-software-dev-demo/research/spend300get30off/proposals.md) (Proposal 1)
- [design_doc_template.md](file:///c:/Users/NCYU/Desktop/陳允瀚/軟工實務助教/115學年/教學計劃/ai-software-dev-demo/prompts/design_doc_template.md)
- [demo.proto](file:///c:/Users/NCYU/Desktop/陳允瀚/軟工實務助教/115學年/教學計劃/ai-software-dev-demo/protos/demo.proto)
