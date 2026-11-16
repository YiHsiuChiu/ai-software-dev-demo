# Codebase 現況研究報告：結帳與運費計算機制 (Free Shipping Feature Research)

## High-level Summary

目前 Online Boutique 系統中的運費估算與訂購流程由 `frontend`、`checkoutservice` 與 `shippingservice` 三個核心服務共同參與。`shippingservice` 僅依據 gRPC 協定接收收件地址與購物車品項清單，其實作不計算品項金額，且無條件回傳固定運費 USD $8.99。`checkoutservice` 與 `frontend` 分別獨立向 `shippingservice` 查詢運費後透過 `currencyservice` 轉匯為使用者貨幣，並將運費與商品小計加總計算訂單總額。現有系統中並不存在任何針對訂單金額滿額（如 US$200）減免運費的判斷邏輯，且 `shippingservice` 的介面與實作均未獲取訂單商品金額或總價資訊。

---

## Detailed Findings

### 1. gRPC 協定定義 (`protos/demo.proto`)

- **運費服務介面**：
  - `ShippingService` 定義了 `GetQuote` 與 `ShipOrder` 兩個 RPC（`protos/demo.proto:107-110`）。
  - `GetQuoteRequest` 僅包含 `Address address` 與 `repeated CartItem items`（`protos/demo.proto:112-115`），並未包含商品單價、幣別或訂單金額。
  - `GetQuoteResponse` 僅回傳單一欄位 `Money cost_usd`（`protos/demo.proto:117-119`）。
  - `ShipOrderRequest` 包含 `Address address` 與 `repeated CartItem items`，回傳 `ShipOrderResponse`（`string tracking_id`）（`protos/demo.proto:121-128`）。
- **結帳服務介面**：
  - `CheckoutService` 定義了 `PlaceOrder(PlaceOrderRequest) returns (PlaceOrderResponse)`（`protos/demo.proto:224-226`）。
  - `PlaceOrderRequest` 傳入 `user_id`、`user_currency`、`address`、`email` 以及 `credit_card`（`protos/demo.proto:228-235`）。
  - `PlaceOrderResponse` 回傳 `OrderResult`（`protos/demo.proto:237-239`），其中 `OrderResult` 包含 `shipping_tracking_id`、`Money shipping_cost`、`Address shipping_address` 及 `repeated OrderItem items`（`protos/demo.proto:208-214`）。
- **金額表示**：
  - `Money` 結構包含 `currency_code`、`int64 units` 與 `int32 nanos`（`protos/demo.proto:146-161`）。

### 2. 運費服務實作 (`src/shippingservice/`)

- **運費計算邏輯**：
  - 在 `src/shippingservice/main.go:117-132`，`GetQuote` 方法接收 `GetQuoteRequest`，但實作直接呼叫 `CreateQuoteFromCount(0)`，完全忽略傳入的 `Address` 與 `Items`。
  - `CreateQuoteFromCount` 定義於 `src/shippingservice/quote.go:34-36`，其內部硬編碼呼叫 `CreateQuoteFromFloat(8.99)`，回傳固定為 8 元 99 分的 `Quote` 結構。
  - `GetQuote` 將此報價包裝為 `CostUsd: &pb.Money{CurrencyCode: "USD", Units: 8, Nanos: 990000000}` 回傳給呼叫端（`src/shippingservice/main.go:126-130`）。
- **訂單發貨邏輯**：
  - `ShipOrder` 接收收件地址與品項，呼叫 `CreateTrackingId(baseAddress)` 產生由地址與隨機數組成的虛擬追蹤碼（`src/shippingservice/main.go:136-147`）。
- **既有單元測試**：
  - `src/shippingservice/shippingservice_test.go:26-56` 的 `TestGetQuote` 驗證回傳的運費為固定值（`Units == 8` 且 `Nanos == 990000000`）。

### 3. 結帳編排服務實作 (`src/checkoutservice/`)

- **結帳主流程 (`PlaceOrder`)**：
  - 於 `src/checkoutservice/main.go:229-279` 執行訂單處理：
    1. 呼叫 `cs.prepareOrderItemsAndShippingQuoteFromCart` 取得品項資料與換匯後的運費（`src/checkoutservice/main.go:237`）。
    2. 初始化訂單總額 `total`，幣別設為 `req.UserCurrency`，先加上運費 `*prep.shippingCostLocalized`（`src/checkoutservice/main.go:242-245`）。
    3. 巡訪各品項，將品項單價乘以數量（`money.MultiplySlow`）後累加至 `total`（`src/checkoutservice/main.go:246-249`）。
    4. 將計算出的 `total` 傳入 `cs.chargeCard` 向 `paymentservice` 發起信用卡扣款（`src/checkoutservice/main.go:251-255`）。
    5. 呼叫 `cs.shipOrder` 向 `shippingservice` 取得物流追蹤碼（`src/checkoutservice/main.go:257-260`）。
    6. 呼叫 `cs.emptyUserCart` 清空購物車（`src/checkoutservice/main.go:262`）。
    7. 組裝 `OrderResult`，包含運費 `ShippingCost: prep.shippingCostLocalized`（`src/checkoutservice/main.go:264-270`）。
    8. 呼叫 `cs.sendOrderConfirmation` 透過 `emailservice` 發送訂單確認信（`src/checkoutservice/main.go:272-276`）。
- **運費與品項準備邏輯 (`prepareOrderItemsAndShippingQuoteFromCart`)**：
  - 定義於 `src/checkoutservice/main.go:287-310`：
    1. 呼叫 `cs.getUserCart` 向 `cartservice` 取得 `[]*pb.CartItem`（`src/checkoutservice/main.go:289`）。
    2. 呼叫 `cs.prepOrderItems` 向 `productcatalogservice` 取得商品資料並以 `currencyservice` 轉為 `userCurrency`（`src/checkoutservice/main.go:293`）。
    3. 呼叫 `cs.quoteShipping` 向 `shippingservice.GetQuote` 取得運費 `shippingUSD`（`src/checkoutservice/main.go:297`）。
    4. 呼叫 `cs.convertCurrency` 將 `shippingUSD` 轉為 `userCurrency`，存入 `out.shippingCostLocalized`（`src/checkoutservice/main.go:301-306`）。
- **Money 輔助計算模組**：
  - `src/checkoutservice/money/money.go:1-132` 提供了 `Sum`、`MultiplySlow`、`IsValid`、`IsZero`、`IsPositive` 等運算，但現有程式碼未實作大小比較（如 `GreaterThan`、`Compare`）函式。

### 4. 前端展示與流程實作 (`src/frontend/`)

- **購物車畫面顯示 (`viewCartHandler`)**：
  - 定義於 `src/frontend/handlers.go:251-318`：
    1. 呼叫 `fe.getCart` 取得使用者購物車品項（`src/frontend/handlers.go:259`）。
    2. 呼叫 `fe.getShippingQuote`（`src/frontend/handlers.go:271`），內部直接呼叫 `shippingservice.GetQuote(&pb.GetQuoteRequest{Address: nil, Items: items})`，再呼叫 `currencyservice.Convert` 將 USD 轉換為目前選定幣別（`src/frontend/rpc.go:87-97`）。
    3. 取得每個商品資訊並將商品美金售價轉為目前選定幣別，累加商品總金額至 `totalPrice`（`src/frontend/handlers.go:284-302`）。
    4. 將運費加至總金額：`totalPrice = money.Must(money.Sum(totalPrice, *shippingCost))`（`src/frontend/handlers.go:303`）。
    5. 傳入模板 `cart.html`（`src/frontend/handlers.go:306-317`），模板直接顯示 `Shipping`（`renderMoney .shipping_cost`）與 `Total`（`renderMoney .total_cost`）（`src/frontend/templates/cart.html:88-96`）。
- **送出訂單流程 (`placeOrderHandler`)**：
  - 定義於 `src/frontend/handlers.go:320-400`：
    1. 驗證表單輸入欄位（`src/frontend/handlers.go:337-352`）。
    2. 呼叫 `checkoutservice.PlaceOrder`（`src/frontend/handlers.go:354-370`）。
    3. 收到回應後，以回傳之 `order.GetOrder().GetShippingCost()` 為基底，加總各品項金額得到 `totalPaid`（`src/frontend/handlers.go:380-384`）。
    4. 渲染完成頁面 `order.html`，顯示追蹤碼與總支付金額（`src/frontend/handlers.go:392-400`，`src/frontend/templates/order.html:48-63`）。
- **橫幅促銷訊息**：
  - 導覽列頂部存在免費運費相關的 HTML 結構 `<div class="h-free-shipping">{{ $.frontendMessage }}</div>`（`src/frontend/templates/header.html:51-57`）。
  - 其內容由環境變數 `FRONTEND_MESSAGE` 控制（`src/frontend/handlers.go:46, 561`），目前在 `compose.yaml` 中未設定該環境變數。

### 5. 相關輔助微服務實作現況

- **`currencyservice` (`src/currencyservice/server.js:138-165`)**：
  - 負責幣別換算，使用固定於 `src/currencyservice/data/currency_conversion.json` 的歐洲央行匯率表。
  - 將來源幣別轉換為歐元（EUR），再轉換為目標幣別（`to_code`）。
- **`productcatalogservice` (`src/productcatalogservice/products.json:1-112`)**：
  - 儲存所有商品原始資料，所有商品價格皆直接以 USD（`priceUsd`）定義。
- **`cartservice` (`src/cartservice/src/services/CartService.cs:34-50`)**：
  - 僅負責記錄 `user_id`、`product_id` 與 `quantity`，不保存或計算任何價格。
- **`paymentservice` (`src/paymentservice/charge.js:61-86`)**：
  - 驗證信用卡號有效性與到期日，接收由 `checkoutservice` 計算之 `amount`，僅做 log 記錄並回傳隨機交易 ID，不驗證金額邏輯。
- **`emailservice` (`src/emailservice/templates/confirmation.html:33-36`)**：
  - 接收 `OrderResult`，模板直接渲染 `order.shipping_cost.units` 與 `order.shipping_cost.nanos`。

---

## Component 關聯與資料流（Data Flow）

### 購物車頁面運費檢視流程 (`GET /cart`)

```
[Browser]
    │  GET /cart
    ▼
[frontend: viewCartHandler]
    ├─► [cartservice: GetCart] ───────────────► 取得品項與數量 (CartItem[])
    ├─► [shippingservice: GetQuote] ──────────► 回傳固定 USD $8.99
    ├─► [currencyservice: Convert] ───────────► 將 USD $8.99 轉為目前幣別 (shippingCost)
    ├─► [productcatalogservice: GetProduct] ──► 取得商品 USD 售價
    ├─► [currencyservice: Convert] ───────────► 將商品售價轉為目前幣別
    │   └─ 計算 items 小計並加上 shippingCost 得到 total_cost
    ▼
[Render templates/cart.html] (展示 shipping_cost 與 total_cost)
```

### 訂單結帳流程 (`POST /cart/checkout`)

```
[Browser]
    │  POST /cart/checkout (Address, CreditCard, etc.)
    ▼
[frontend: placeOrderHandler]
    │
    ▼ (gRPC PlaceOrder)
[checkoutservice: PlaceOrder]
    ├─► [cartservice: GetCart] ───────────────► 取得 CartItem[]
    ├─► [productcatalogservice: GetProduct] ──► 取得商品 USD 售價
    ├─► [currencyservice: Convert] ───────────► 各商品轉為 userCurrency 並計算商品小計
    ├─► [shippingservice: GetQuote] ──────────► 回傳固定 USD $8.99
    ├─► [currencyservice: Convert] ───────────► 將運費轉為 userCurrency (shippingCostLocalized)
    │   └─ 加總商品總額與 shippingCostLocalized 得到 total
    ├─► [paymentservice: Charge] ─────────────► 依據 total 扣款
    ├─► [shippingservice: ShipOrder] ─────────► 建立物流訂單並取得 tracking_id
    ├─► [cartservice: EmptyCart] ─────────────► 清空購物車
    ├─► [emailservice: SendOrderConfirmation] ► 發送訂單確認信 (帶有 OrderResult)
    │
    ▼ 回傳 PlaceOrderResponse (OrderResult)
[frontend: placeOrderHandler]
    │  根據 order.ShippingCost 與 items 重新計算 totalPaid
    ▼
[Render templates/order.html] (展示訂單編號、物流追蹤碼與支付總額)
```

---

## Code References

| 元件 / 模組 | 檔案路徑與行號 | 說明 |
| :--- | :--- | :--- |
| **Proto 協定** | `protos/demo.proto:107-136` | `ShippingService` 介面與 `GetQuoteRequest` / `GetQuoteResponse` 定義 |
| **Proto 協定** | `protos/demo.proto:208-240` | `CheckoutService` 介面、`PlaceOrderRequest` 與 `OrderResult` 定義 |
| **Proto 協定** | `protos/demo.proto:146-161` | `Money` 結構定義 |
| **Shipping** | `src/shippingservice/main.go:116-132` | `GetQuote` 實作：忽略輸入直接呼叫 `CreateQuoteFromCount(0)` |
| **Shipping** | `src/shippingservice/quote.go:34-45` | `CreateQuoteFromCount` 硬編碼回傳 `$8.99` 運費 |
| **Shipping 測試** | `src/shippingservice/shippingservice_test.go:26-56` | `TestGetQuote` 單元測試斷言運費固定為 8 元 990000000 奈米 |
| **Checkout** | `src/checkoutservice/main.go:229-279` | `PlaceOrder` 流程：取得運費、累加品項金額、扣款與建立訂單 |
| **Checkout** | `src/checkoutservice/main.go:287-310` | `prepareOrderItemsAndShippingQuoteFromCart` 運費查詢與換匯 |
| **Checkout** | `src/checkoutservice/main.go:312-321` | `quoteShipping` 呼叫 `shippingservice.GetQuote` |
| **Checkout** | `src/checkoutservice/main.go:338-356` | `prepOrderItems` 呼叫商品目錄服務並換匯 |
| **Money 工具** | `src/checkoutservice/money/money.go:93-132` | 金額加總 `Sum` 與乘法 `MultiplySlow` 函式 |
| **Frontend** | `src/frontend/handlers.go:251-318` | `viewCartHandler` 購物車運費查詢與總額加總 |
| **Frontend** | `src/frontend/rpc.go:87-97` | `getShippingQuote` 呼叫運費服務與貨幣轉換 |
| **Frontend** | `src/frontend/handlers.go:320-400` | `placeOrderHandler` 結帳請求處理與付款總額呈現 |
| **Frontend 模板** | `src/frontend/templates/cart.html:88-96` | 購物車畫面運費及總價呈現欄位 |
| **Frontend 模板** | `src/frontend/templates/order.html:48-63` | 訂單完成畫面物流追蹤碼與支付總額欄位 |
| **Frontend 模板** | `src/frontend/templates/header.html:51-57` | 免費運費橫幅模板標籤與變數綁定 |
| **Currency** | `src/currencyservice/server.js:138-165` | `convert` 貨幣轉換實作 |
| **Cart** | `src/cartservice/src/services/CartService.cs:34-50` | 購物車儲存品項實作（不含價格） |
| **Payment** | `src/paymentservice/charge.js:61-86` | 信用卡扣款實作（僅記錄金額並回傳隨機 transaction ID） |
| **Email** | `src/emailservice/templates/confirmation.html:33-36` | 訂單確認信中運費與物流追蹤碼模板 |
