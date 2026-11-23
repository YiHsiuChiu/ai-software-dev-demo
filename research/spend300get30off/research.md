# Codebase 現況研究報告：訂單金額折扣與免運計算現況 (Spend $300 Get $30 Off Feature Research)

## High-level Summary

目前 Online Boutique 系統中的結帳與運費計算由 `frontend`、`checkoutservice` 與 `shippingservice` 三個核心服務協同運作。既有實作已於 `shippingservice.GetQuote` 支援依據 `GetQuoteRequest.items_subtotal_usd` 判斷商品金額是否達 200 USD，若滿額則回傳 0 元運費，否則回傳固定運費 8.99 USD。在目前的結帳與購物車流程中，`frontend` 與 `checkoutservice` 均直接計算商品小計並累加運費作為訂單總額，系統內完全不存在任何折扣（Discount）或滿額折抵的資料結構、計算邏輯與展示介面。此外，`protos/demo.proto` 中的 `OrderResult` 僅包含追蹤碼、運費、地址與品項列表，並無任何折扣金額或折抵後小計欄位，現有程式碼亦無支援金額扣減或大小比較的公用函式。

---

## Detailed Findings

### 1. gRPC 協定定義 (`protos/demo.proto`)

- **運費服務介面 (`ShippingService`)**：
  - 定義於 `protos/demo.proto:107-110`，提供 `GetQuote` 與 `ShipOrder` 兩個 RPC。
  - `GetQuoteRequest`（`protos/demo.proto:112-116`）包含：
    - `Address address = 1;`
    - `repeated CartItem items = 2;`
    - `Money items_subtotal_usd = 3;`（用於免運費門檻判斷之美金商品小計）
  - `GetQuoteResponse`（`protos/demo.proto:118-120`）僅包含 `Money cost_usd = 1;`。
- **結帳服務介面 (`CheckoutService`)**：
  - 定義於 `protos/demo.proto:225-227`，提供 `PlaceOrder(PlaceOrderRequest) returns (PlaceOrderResponse)`。
  - `PlaceOrderRequest`（`protos/demo.proto:229-236`）包含 `user_id`、`user_currency`、`address`、`email` 以及 `credit_card`，無任何優惠券或折扣欄位。
  - `PlaceOrderResponse`（`protos/demo.proto:238-240`）包含 `OrderResult order = 1;`。
- **訂單結果與金額結構 (`OrderResult` & `Money`)**：
  - `OrderResult`（`protos/demo.proto:209-215`）定義包含：
    - `string order_id = 1;`
    - `string shipping_tracking_id = 2;`
    - `Money shipping_cost = 3;`
    - `Address shipping_address = 4;`
    - `repeated OrderItem items = 5;`
    - 現有結構中無 `discount`、`subtotal` 或 `total` 欄位。
  - `Money`（`protos/demo.proto:147-162`）包含 `string currency_code = 1;`、`int64 units = 2;`、`int32 nanos = 3;`。
  - `OrderItem`（`protos/demo.proto:204-207`）包含 `CartItem item = 1;` 與 `Money cost = 2;`。

### 2. 運費計算與免運費實作 (`src/shippingservice/`)

- **報價計算邏輯 (`GetQuote`)**：
  - 定義於 `src/shippingservice/main.go:117-145`。
  - 免運門檻判斷實作於 `src/shippingservice/main.go:121-132`：
    ```go
    if subtotal := in.GetItemsSubtotalUsd(); subtotal != nil {
        if subtotal.GetUnits() > 200 || (subtotal.GetUnits() == 200 && subtotal.GetNanos() >= 0) {
            return &pb.GetQuoteResponse{
                CostUsd: &pb.Money{
                    CurrencyCode: "USD",
                    Units:        0,
                    Nanos:        0,
                },
            }, nil
        }
    }
    ```
  - 當傳入的 `ItemsSubtotalUsd` 未達 200 USD 或為 `nil` 時，呼叫 `CreateQuoteFromCount(0)`，回傳固定運費 8.99 USD（`src/shippingservice/main.go:134-143`、`src/shippingservice/quote.go:34-45`）。
- **單元測試實作**：
  - 於 `src/shippingservice/shippingservice_test.go:25-107`（`TestGetQuote`），涵蓋未提供 subtotal（回傳 8.99 USD）、subtotal 未達 $200（199.99 USD 回傳 8.99 USD）、剛好達 $200（200.00 USD 回傳 0 USD）以及超過 $200（250.00 USD 回傳 0 USD）等情況。

### 3. 結帳編排服務實作 (`src/checkoutservice/`)

- **訂單處理流程 (`PlaceOrder`)**：
  - 定義於 `src/checkoutservice/main.go:229-279`。
  - 呼叫 `cs.prepareOrderItemsAndShippingQuoteFromCart` 取得訂單項目與運費報價（`src/checkoutservice/main.go:237`）。
  - 計算扣款總金額 `total`（`src/checkoutservice/main.go:242-249`）：
    ```go
    total := pb.Money{CurrencyCode: req.UserCurrency,
        Units: 0,
        Nanos: 0}
    total = money.Must(money.Sum(total, *prep.shippingCostLocalized))
    for _, it := range prep.orderItems {
        multPrice := money.MultiplySlow(*it.Cost, uint32(it.GetItem().GetQuantity()))
        total = money.Must(money.Sum(total, multPrice))
    }
    ```
    計算邏輯為：將本地化運費與各品項總價（單價 × 數量）直接相加，未扣除任何折扣。
  - 扣款信用卡（`src/checkoutservice/main.go:251-255`）：傳入前述未經折抵之 `total` 至 `paymentservice.Charge`。
  - 發貨與清空購物車（`src/checkoutservice/main.go:257-262`）。
  - 組裝 `OrderResult`（`src/checkoutservice/main.go:264-270`），僅填入 `orderID`、`shippingTrackingID`、`shippingCostLocalized`、`shippingAddress` 與 `orderItems`。
  - 發送確認信件（`src/checkoutservice/main.go:272-276`）。
- **品項與運費報價準備 (`prepareOrderItemsAndShippingQuoteFromCart`)**：
  - 定義於 `src/checkoutservice/main.go:287-310`。
  - 取得購物車品項並準備訂單品項與美金小計（`src/checkoutservice/main.go:289-296`）：
    - 呼叫 `cs.prepOrderItems`，回傳 `orderItems` 與 `itemsSubtotalUSD`。
  - 查詢運費報價（`src/checkoutservice/main.go:297-300`）：
    - 呼叫 `cs.quoteShipping(ctx, address, cartItems, itemsSubtotalUSD)`，內部將未經折抵的原始商品總額 `itemsSubtotalUSD` 作為 `ItemsSubtotalUsd` 傳入 `shippingservice.GetQuote`（`src/checkoutservice/main.go:312-322`）。
  - 換匯運費為使用者幣別（`src/checkoutservice/main.go:301-306`）。
- **品項處理與美金小計計算 (`prepOrderItems`)**：
  - 定義於 `src/checkoutservice/main.go:339-363`。
  - 巡訪購物車品項，向 `productcatalogservice` 取得商品美金單價並以 `currencyservice` 轉為 `userCurrency` 存入 `OrderItem.Cost`。
  - 若品項數量大於 0，累加美金總額（`src/checkoutservice/main.go:357-360`）：
    ```go
    multPrice := money.MultiplySlow(*product.GetPriceUsd(), uint32(item.GetQuantity()))
    subtotalUSD = money.Must(money.Sum(subtotalUSD, multPrice))
    ```

### 4. 前端展示與結帳流程實作 (`src/frontend/`)

- **購物車頁面處理 (`viewCartHandler`)**：
  - 定義於 `src/frontend/handlers.go:251-324`。
  - 取得購物車品項後，逐項查詢商品並計算各品項換匯後金額，累加至 `totalPrice`（`src/frontend/handlers.go:279-297`）。
  - 同時計算品項之原始 USD 金額累加至 `itemsUSD`（`src/frontend/handlers.go:298-301`）：
    ```go
    if item.GetQuantity() > 0 {
        multPriceUSD := money.MultiplySlow(*p.GetPriceUsd(), uint32(item.GetQuantity()))
        itemsUSD = money.Must(money.Sum(itemsUSD, multPriceUSD))
    }
    ```
  - 取得運費報價（`src/frontend/handlers.go:304-308`）：呼叫 `fe.getShippingQuote(r.Context(), cart, &itemsUSD, currentCurrency(r))`，傳入未扣除任何折扣的 `itemsUSD`。
  - `fe.getShippingQuote`（`src/frontend/rpc.go:87-98`）發送 `GetQuoteRequest{ Address: nil, Items: items, ItemsSubtotalUsd: subtotalUsd }` 至 `shippingservice`，並將回傳運費換匯為目前幣別。
  - 累加總額：`totalPrice = money.Must(money.Sum(totalPrice, *shippingCost))`（`src/frontend/handlers.go:309`）。
  - 傳入模板資料（`src/frontend/handlers.go:312-320`）：包含 `shipping_cost`、`total_cost`、`items` 等，但無任何 discount 欄位。
- **購物車模板渲染 (`src/frontend/templates/cart.html`)**：
  - 於 `src/frontend/templates/cart.html:88-96` 渲染摘要：
    ```html
    <div class="row cart-summary-shipping-row">
        <div class="col pl-md-0">Shipping</div>
        <div class="col pr-md-0 text-right">{{ renderMoney .shipping_cost }}</div>
    </div>

    <div class="row cart-summary-total-row">
        <div class="col pl-md-0">Total</div>
        <div class="col pr-md-0 text-right">{{ renderMoney .total_cost }}</div>
    </div>
    ```
    目前僅有 Shipping 與 Total 兩列，無商品小計（Subtotal）列或折扣（Discount）列。
- **下單送出處理 (`placeOrderHandler`)**：
  - 定義於 `src/frontend/handlers.go:326-407`。
  - 驗證表單後呼叫 `checkoutservice.PlaceOrder`（`src/frontend/handlers.go:360-376`）。
  - 前端自行計算已支付總額 `totalPaid`（`src/frontend/handlers.go:386-390`）：
    ```go
    totalPaid := *order.GetOrder().GetShippingCost()
    for _, v := range order.GetOrder().GetItems() {
        multPrice := money.MultiplySlow(*v.GetCost(), uint32(v.GetItem().GetQuantity()))
        totalPaid = money.Must(money.Sum(totalPaid, multPrice))
    }
    ```
    前端並未直接從 `order` 物件讀取總扣款金額，而是透過運費加總各品項金額自行算出 `totalPaid`。
  - 傳入模板資料並渲染完成頁面（`src/frontend/handlers.go:398-406`）。
- **訂單完成模板渲染 (`src/frontend/templates/order.html`)**：
  - 於 `src/frontend/templates/order.html:40-63` 顯示訂單確認號碼、物流追蹤碼與支付總額（`Total Paid: {{renderMoney .total_paid}}`），無任何折扣金額標示。

### 5. 金額運算模組現況 (`src/checkoutservice/money/` 與 `src/frontend/money/`)

- 兩服務各自具備獨立的 `money` package（`src/checkoutservice/money/money.go:1-132`、`src/frontend/money/money.go:1-132`）。
- 現有實作提供函式清單：
  - `IsValid(m pb.Money) bool`（行 35-37）
  - `IsZero(m pb.Money) bool`（行 46）
  - `IsPositive(m pb.Money) bool`（行 50-52）
  - `IsNegative(m pb.Money) bool`（行 56-58）
  - `AreSameCurrency(l, r pb.Money) bool`（行 62-64）
  - `AreEquals(l, r pb.Money) bool`（行 68-71）
  - `Negate(m pb.Money) pb.Money`（行 74-79）
  - `Must(v pb.Money, err error) pb.Money`（行 83-88）
  - `Sum(l, r pb.Money) (pb.Money, error)`（行 93-121）
  - `MultiplySlow(m pb.Money, n uint32) pb.Money`（行 125-132）
- 現狀缺乏的金額運算工具：
  - 無金額扣減函式（如 `Sub(l, r pb.Money) (pb.Money, error)`，目前僅能透過 `Sum(l, Negate(r))` 達成）。
  - 無兩金額大小比較函式（如 `Compare(l, r pb.Money) int`、`GreaterThanOrEqual`、`GreaterThan`）。

### 6. 相關週邊服務現況

- **幣別轉換服務 (`src/currencyservice/server.js:138-169`)**：
  - 接收 `CurrencyConversionRequest{ from, to_code }`，透過基準幣別 EUR 進行兩階段換算。
  - 支援將 USD 折扣金額或品項金額轉換為使用者選定幣別。
- **商品型錄服務 (`src/productcatalogservice/products.json:1-112`)**：
  - 儲存所有商品資訊，所有商品的 `priceUsd` 均以 USD（`units` 與 `nanos`）定義。
- **購物車服務 (`src/cartservice/src/services/CartService.cs:34-50`)**：
  - 僅負責維護使用者購物車中的 `ProductId` 與 `Quantity`，不儲存商品價格，亦不涉及折扣。
- **支付服務 (`src/paymentservice/charge.js:61-86`)**：
  - 僅驗證信用卡號有效性、卡別（僅接受 VISA 與 Mastercard）與過期日，並 log 接收到的扣款金額（`amount.currency_code`、`amount.units`、`amount.nanos`），不比對訂單或商品品項。
- **郵件服務 (`src/emailservice/templates/confirmation.html:31-51`)**：
  - 渲染 `order.order_id`、`order.shipping_tracking_id`、`order.shipping_cost` 以及各品項名稱、數量與單價，未顯示總額或折扣。

---

## Component 關聯與資料流（Data Flow）

### 購物車頁面檢視流程 (`GET /cart`)

```
[Browser]
    │  GET /cart
    ▼
[frontend: viewCartHandler] (src/frontend/handlers.go:251)
    │
    ├─► cartservice.GetCart (取得購物車 CartItem 清單)
    │
    ├─► productcatalogservice.GetProduct (取得各商品 USD 價格)
    │
    ├─► currencyservice.Convert (將商品價格由 USD 轉為使用者幣別)
    │     └─► 逐項累加：
    │           - totalPrice = sum(localized item cost * quantity)
    │           - itemsUSD = sum(usd item cost * quantity)
    │
    ├─► shippingservice.GetQuote(&pb.GetQuoteRequest{ItemsSubtotalUsd: itemsUSD}) (src/frontend/rpc.go:87)
    │     │
    │     └─► [shippingservice: GetQuote] (src/shippingservice/main.go:117)
    │           ├─► 檢查 items_subtotal_usd 是否 >= 200 USD:
    │           │     - 若 >= 200: 回傳 0 USD
    │           │     - 若 < 200: 回傳固定 8.99 USD
    │           └─► 回傳 CostUsd
    │
    ├─► currencyservice.Convert (將 CostUsd 轉為使用者幣別運費 shippingCost)
    │
    ├─► totalPrice = totalPrice + shippingCost (無任何滿 300 折 30 的計算)
    │
    ▼
[HTML Response: cart.html] (src/frontend/templates/cart.html:88-96)
    - 顯示各品項價格
    - 顯示 Shipping: {{ renderMoney .shipping_cost }}
    - 顯示 Total: {{ renderMoney .total_cost }}
```

### 訂單結帳流程 (`POST /cart/checkout`)

```
[Browser]
    │  POST /cart/checkout
    ▼
[frontend: placeOrderHandler] (src/frontend/handlers.go:326)
    │
    ├─► 驗證使用者表單欄位
    │
    ├─► checkoutservice.PlaceOrder(PlaceOrderRequest) (src/checkoutservice/main.go:229)
    │     │
    │     ├─► prepareOrderItemsAndShippingQuoteFromCart (src/checkoutservice/main.go:287)
    │     │     ├─► cartservice.GetCart
    │     │     ├─► prepOrderItems (計算品項 userCurrency 價格，並累加計算 itemsSubtotalUSD)
    │     │     ├─► quoteShipping (傳遞未折抵之 itemsSubtotalUSD 至 shippingservice.GetQuote)
    │     │     │     └─► shippingservice 判定是否 >= 200 USD (回傳 0 或 8.99 USD)
    │     │     └─► convertCurrency (將運費轉為 userCurrency)
    │     │
    │     ├─► total = shippingCostLocalized + sum(itemCost * quantity) (未計算滿 300 折 30)
    │     │
    │     ├─► paymentservice.Charge(total, credit_card) (依未折抵總額扣款)
    │     ├─► shippingservice.ShipOrder (取得物流追蹤碼)
    │     ├─► cartservice.EmptyCart (清空購物車)
    │     ├─► emailservice.SendOrderConfirmation (發送確認信)
    │     │
    │     └─► 回傳 OrderResult (包含 OrderId, ShippingCost, Items，無 discount 欄位)
    │
    ├─► 前端自行重新計算支付總額：
    │     totalPaid = order.ShippingCost + sum(item.Cost * item.Quantity) (無折抵扣減)
    │
    ▼
[HTML Response: order.html] (src/frontend/templates/order.html:40-63)
    - 顯示 Confirmation #, Tracking #
    - 顯示 Total Paid: {{ renderMoney .total_paid }}
```

### 現有免運費與金額計算交互關係

1. **小計金額流向**：`frontend`（`src/frontend/handlers.go:298-301`）與 `checkoutservice`（`src/checkoutservice/main.go:357-360`）皆是在巡訪商品清單時，直接以商品原價 USD 累加出 `itemsUSD` / `itemsSubtotalUSD`。
2. **運費判定依據**：`shippingservice.GetQuote`（`src/shippingservice/main.go:121-132`）直接檢驗傳入之 `ItemsSubtotalUsd` 是否大於等於 200 USD。
3. **無折扣扣除機制**：目前送往 `shippingservice.GetQuote` 的金額未經過任何折扣運算，且結帳金額加總亦無減去折扣之邏輯。
