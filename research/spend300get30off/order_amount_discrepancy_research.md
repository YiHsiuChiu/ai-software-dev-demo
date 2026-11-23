# Codebase 現況研究報告：購物車預估金額與結帳訂單金額差異實作現況 (Order Amount Discrepancy Research)

## High-level Summary

本研究針對目前系統中購物車頁面預估金額與結帳完成頁面顯示金額不一致的現有實作進行全盤比對。在購物車頁面（`viewCartHandler`），前端在本地 Go 程式碼中自行計算商品 USD 小計，滿 300 USD 時直接計算 30 USD 折扣並從 `totalPrice` 扣除，故購物車呈現折抵後之 299.97 USD。在結帳階段，後端 `checkoutservice` 同樣計算了 30 USD 折扣並依折抵後之 299.97 USD 向 `paymentservice` 發起信用卡扣款，且將該折扣賦值給 `OrderResult.Discount`。然而在 gRPC 傳輸層，產生的 `demo.pb.go` 內部編譯描述子（`file_demo_proto_rawDesc` 與 `file_demo_proto_depIdxs`）中未包含 `OrderResult` 第 6 號欄位 `discount` 的中繼資料，導致前端接收到的 `order.GetOrder().GetDiscount()` 為 `nil`；前端 `placeOrderHandler` 依據 `ShippingCost` 與 `Items` 自行重新加總計算 `totalPaid`，在未取得折扣物件時未進行扣除，最終在完成頁面呈現未扣除折扣之 329.97 USD。

---

## Detailed Findings

### 1. 前端購物車頁面金額計算與展示實作 (`src/frontend/`)

- **購物車金額計算邏輯 (`viewCartHandler`)**：
  - 定義於 `src/frontend/handlers.go:251-335`。
  - 於 `src/frontend/handlers.go:276-302` 巡訪購物車品項：
    - 計算各品項換匯後價格並累加至 `totalPrice`（`src/frontend/handlers.go:296`）。
    - 累加商品 USD 原始價格至 `itemsUSD`（`src/frontend/handlers.go:298-301`）。
    - 購買 3 支單價 109.99 USD 的手錶時，`itemsUSD` 與 `totalPrice` 累計皆為 329.97 USD。
  - 於 `src/frontend/handlers.go:304-319` 執行滿額折扣判定：
    - 定義 `discountThresholdUSD` 為 300 USD，`discountUSD` 為 30 USD。
    - 透過 `money.Compare(itemsUSD, discountThresholdUSD)` 判定：因為 329.97 $\ge$ 300，`cmp >= 0` 成立。
    - 計算折後小計：`discountedSubtotalUSD = money.Must(money.Sub(itemsUSD, discountUSD))`，數值為 299.97 USD。
    - 將 30 USD 換匯為當前幣別存入 `discountCost`。
  - 於 `src/frontend/handlers.go:320-324` 查詢運費：
    - 呼叫 `fe.getShippingQuote(r.Context(), cart, &discountedSubtotalUSD, currentCurrency(r))`。
    - 傳入之折後金額為 299.97 USD，因大於 200 USD，`shippingservice.GetQuote` 回傳運費 0 USD。
  - 於 `src/frontend/handlers.go:325-328` 計算總額：
    - 執行 `totalPrice = money.Must(money.Sub(totalPrice, *discountCost))`，扣減 30 USD 折扣（329.97 - 30.00 = 299.97）。
    - 加上運費 0 USD：`totalPrice = money.Must(money.Sum(totalPrice, *shippingCost))`，總額維持 299.97 USD。
  - 於 `src/frontend/handlers.go:331-338` 注入模板資料：
    - 傳入 `"discount": discountCost`（30.00 USD）與 `"total_cost": totalPrice`（299.97 USD）。
- **購物車模板渲染 (`src/frontend/templates/cart.html`)**：
  - 定義於 `src/frontend/templates/cart.html:88-100`。
  - 行 93-98 檢查 `.discount`：
    ```html
    {{ if and .discount (or (gt .discount.GetUnits 0) (gt .discount.GetNanos 0)) }}
    <div class="row cart-summary-discount-row text-success">
        <div class="col pl-md-0">Discount (Orders $300+)</div>
        <div class="col pr-md-0 text-right">- {{ renderMoney .discount }}</div>
    </div>
    {{ end }}
    ```
    因為 `.discount` 為 30.00 USD，此區塊被渲染，顯示 `- $30.00`。
  - 行 99-102 渲染總額：
    ```html
    <div class="row cart-summary-total-row">
        <div class="col pl-md-0">Total</div>
        <div class="col pr-md-0 text-right">{{ renderMoney .total_cost }}</div>
    </div>
    ```
    渲染結果顯示 `Total: $299.97`。

---

### 2. 結帳編排服務實作 (`src/checkoutservice/`)

- **訂單前置準備 (`prepareOrderItemsAndShippingQuoteFromCart`)**：
  - 定義於 `src/checkoutservice/main.go:292-334`。
  - 於 `src/checkoutservice/main.go:298` 呼叫 `cs.prepOrderItems`，回傳 `itemsSubtotalUSD` 為 329.97 USD。
  - 於 `src/checkoutservice/main.go:303-318` 執行折扣判定：
    - 定義 `discountThresholdUSD` 為 300 USD，`discountUSD` 為 30 USD。
    - 透過 `money.Compare(*itemsSubtotalUSD, discountThresholdUSD)` 判定成立（`cmp >= 0`）。
    - 計算 `discountedSubtotalUSD = itemsSubtotalUSD - 30 USD = 299.97 USD`。
    - 將 30 USD 轉換為 `userCurrency` 存入 `discountLocalized`（30.00 USD）。
  - 於 `src/checkoutservice/main.go:320-327` 呼叫 `cs.quoteShipping` 取得運費 0 USD，並轉匯為 `shippingCostLocalized`（0.00 USD）。
  - 於 `src/checkoutservice/main.go:329-333` 封裝 `orderPrep`：
    - `out.shippingCostLocalized = 0.00 USD`
    - `out.discountLocalized = 30.00 USD`
- **下單與信用卡扣款流程 (`PlaceOrder`)**：
  - 定義於 `src/checkoutservice/main.go:229-283`。
  - 於 `src/checkoutservice/main.go:242-252` 計算扣款總額 `total`：
    - 初始化 `total` 為 0 USD，累加運費 0 USD。
    - 走訪 `prep.orderItems` 累加各品項金額（109.99 × 3 = 329.97 USD）。
    - 執行折扣扣減：
      ```go
      if prep.discountLocalized != nil && !money.IsZero(*prep.discountLocalized) {
          total = money.Must(money.Sub(total, *prep.discountLocalized))
      }
      ```
      扣減 30 USD 折扣後，`total` 為 299.97 USD。
  - 於 `src/checkoutservice/main.go:254-257` 呼叫 `cs.chargeCard(ctx, &total, req.CreditCard)`：
    - 向 `paymentservice` 發起的請款金額實為 299.97 USD。
  - 於 `src/checkoutservice/main.go:267-274` 組裝 `OrderResult`：
    ```go
    orderResult := &pb.OrderResult{
        OrderId:            orderID.String(),
        ShippingTrackingId: shippingTrackingID,
        ShippingCost:       prep.shippingCostLocalized,
        ShippingAddress:    req.Address,
        Items:              prep.orderItems,
        Discount:           prep.discountLocalized,
    }
    ```
    此處 `Discount` 欄位被指派為 `prep.discountLocalized`（30.00 USD）。
  - 於 `src/checkoutservice/main.go:281` 回傳 `&pb.PlaceOrderResponse{Order: orderResult}`。

---

### 3. gRPC 協定定義與 Protobuf 程式碼現況 (`protos/demo.proto` 與 `genproto/demo.pb.go`)

- **Proto 定義現況 (`protos/demo.proto`)**：
  - 於 `protos/demo.proto:209-216`，`OrderResult` 已宣告 6 個欄位：
    ```protobuf
    message OrderResult {
        string   order_id = 1;
        string   shipping_tracking_id = 2;
        Money shipping_cost = 3;
        Address  shipping_address = 4;
        repeated OrderItem items = 5;
        Money discount = 6;
    }
    ```
- **Go 語言 Protobuf 結構現況 (`genproto/demo.pb.go`)**：
  - 在 `src/checkoutservice/genproto/demo.pb.go:1409-1420` 與 `src/frontend/genproto/demo.pb.go:1409-1420`：
    - Go struct `OrderResult` 包含欄位：
      ```go
      Discount *Money `protobuf:"bytes,6,opt,name=discount,proto3" json:"discount,omitempty"`
      ```
    - 包含 Getter 方法（行 1488-1493）：
      ```go
      func (x *OrderResult) GetDiscount() *Money {
          if x != nil {
              return x.Discount
          }
          return nil
      }
      ```
- **Protobuf 二進位描述子現況 (`file_demo_proto_rawDesc` & `file_demo_proto_depIdxs`)**：
  - 在 `src/checkoutservice/genproto/demo.pb.go:1831-2130` 與 `src/frontend/genproto/demo.pb.go:1831-2130`：
    - 變數 `file_demo_proto_rawDesc` 儲存由 `protoc` 編譯產生之 Protobuf 描述子位元組陣列。
  - 在 `src/checkoutservice/genproto/demo.pb.go:2160-2210`（`file_demo_proto_depIdxs`）：
    - 行 2177-2179 記載 `OrderResult` 相關之型態關聯索引：
      ```go
      18, // 16: hipstershop.OrderResult.shipping_cost:type_name -> hipstershop.Money
      17, // 17: hipstershop.OrderResult.shipping_address:type_name -> hipstershop.Address
      24, // 18: hipstershop.OrderResult.items:type_name -> hipstershop.OrderItem
      ```
    - 該索引中僅存在欄位 3（`shipping_cost`）、欄位 4（`shipping_address`）與欄位 5（`items`），並未包含欄位 6 `discount` 對應至 `hipstershop.Money` 的描述子關聯。
  - 於 `src/checkoutservice/genproto/demo.pb.go:2590-2631` 的 `file_demo_proto_init` 函式中，Google Protobuf v2 執行階段透過 `protoimpl.TypeBuilder` 依據 `file_demo_proto_rawDesc` 解析訊息結構，未註冊之欄位在 gRPC 序列化與反序列化時不會被映射至接收端結構的欄位中。

---

### 4. 前端訂單結帳與完成頁面實作 (`src/frontend/`)

- **訂單送出與已付金額計算 (`placeOrderHandler`)**：
  - 定義於 `src/frontend/handlers.go:326-430`。
  - 於 `src/frontend/handlers.go:380-396` 呼叫 `checkoutservice.PlaceOrder`。
  - 於 `src/frontend/handlers.go:406-410` 計算實付總額 `totalPaid`：
    ```go
    totalPaid := *order.GetOrder().GetShippingCost()
    for _, v := range order.GetOrder().GetItems() {
        multPrice := money.MultiplySlow(*v.GetCost(), uint32(v.GetItem().GetQuantity()))
        totalPaid = money.Must(money.Sum(totalPaid, multPrice))
    }
    ```
    此處以 `ShippingCost`（0.00 USD）加上各品項金額總和（109.99 × 3 = 329.97 USD），此時 `totalPaid` 為 329.97 USD。
  - 於 `src/frontend/handlers.go:411-413` 檢查折扣扣減：
    ```go
    if d := order.GetOrder().GetDiscount(); d != nil && !money.IsZero(*d) {
        totalPaid = money.Must(money.Sub(totalPaid, *d))
    }
    ```
    當從 gRPC 收到之 `order.GetOrder().GetDiscount()` 為 `nil` 時，此條件不成立，`totalPaid` 未扣減 30 USD，維持 329.97 USD。
  - 於 `src/frontend/handlers.go:421-429` 渲染模板：傳入 `"order": order.GetOrder()` 與 `"total_paid": &totalPaid`（329.97 USD）。
- **訂單完成模板渲染 (`src/frontend/templates/order.html`)**：
  - 定義於 `src/frontend/templates/order.html:40-75`。
  - 於 `src/frontend/templates/order.html:56-65` 條件渲染折扣行：
    ```html
    {{ if and .order.Discount (or (gt .order.Discount.GetUnits 0) (gt .order.Discount.GetNanos 0)) }}
    <div class="row border-bottom-solid padding-y-24">
        <div class="col-6 pl-md-0">
            Discount
        </div>
        <div class="col-6 pr-md-0 text-right text-success">
            - {{renderMoney .order.Discount}}
        </div>
    </div>
    {{ end }}
    ```
    當 `.order.Discount` 為 `nil` 時，整段折扣行被忽略，不予顯示。
  - 於 `src/frontend/templates/order.html:66-73` 渲染實付金額：
    ```html
    <div class="row padding-y-24">
        <div class="col-6 pl-md-0">
            Total Paid
        </div>
        <div class="col-6 pr-md-0 text-right">
            {{renderMoney .total_paid}}
        </div>
    </div>
    ```
    顯示結果為 `Total Paid: $329.97`。

---

## Component 關聯與資料流（Data Flow）

### 購物車頁面金額流程 (`GET /cart`)

```
[Browser]
    │  GET /cart
    ▼
[frontend: viewCartHandler] (src/frontend/handlers.go:251)
    ├─► 讀取商品價格累加 itemsUSD = $329.97
    ├─► 判定 itemsUSD >= $300 成立:
    │     - discountUSD = $30.00
    │     - discountedSubtotalUSD = $299.97
    │     - discountCost = $30.00
    ├─► 呼叫 shippingservice.GetQuote(&pb.GetQuoteRequest{ItemsSubtotalUsd: $299.97})
    │     └─► 回傳運費 $0.00 (因為 $299.97 >= $200)
    ├─► 計算 totalPrice:
    │     $329.97 - $30.00 (discountCost) + $0.00 (shippingCost) = $299.97
    ▼
[cart.html] (src/frontend/templates/cart.html:88-100)
    - 顯示 Discount (Orders $300+): - $30.00
    - 顯示 Shipping: $0.00
    - 顯示 Total: $299.97
```

### 訂單下單結帳流程 (`POST /cart/checkout`)

```
[Browser]
    │  POST /cart/checkout
    ▼
[frontend: placeOrderHandler] (src/frontend/handlers.go:326)
    │  呼叫 checkoutservice.PlaceOrder(req)
    ▼
[checkoutservice: PlaceOrder] (src/checkoutservice/main.go:229)
    ├─► prepareOrderItemsAndShippingQuoteFromCart:
    │     - itemsSubtotalUSD = $329.97
    │     - 判定 >= $300 成立，設定 discountLocalized = $30.00
    │     - 以折後 $299.97 查詢運費，取得 shippingCostLocalized = $0.00
    ├─► 計算總請款金額 total:
    │     $329.97 - $30.00 + $0.00 = $299.97
    ├─► paymentservice.Charge:
    │     以 $299.97 請款信用卡成功
    ├─► 組裝 OrderResult:
    │     OrderResult{ ShippingCost: $0.00, Items: [...], Discount: $30.00 }
    │
    │  透過 gRPC 將 OrderResult 回傳給 frontend
    │  (因 demo.pb.go 二進位描述子未含 discount 欄位描述，反序列化後 Discount 欄位未填入)
    ▼
[frontend: placeOrderHandler] (src/frontend/handlers.go:406)
    ├─► 接收到 order.GetOrder() (其 GetDiscount() 回傳 nil)
    ├─► 自行重新加總計算 totalPaid:
    │     totalPaid = shippingCost ($0.00) + sum(itemCost) ($329.97) = $329.97
    ├─► 檢查 if d := order.GetOrder().GetDiscount(); d != nil
    │     (條件不成立，未執行扣減，totalPaid 維持 $329.97)
    ▼
[order.html] (src/frontend/templates/order.html:56-73)
    - 折扣區塊因 .order.Discount 為 nil 而略過
    - 顯示 Total Paid: $329.97
```

---

## Code References

- `src/frontend/handlers.go:251-335` (`viewCartHandler` 購物車金額與折扣計算)
- `src/frontend/handlers.go:304-328` (`viewCartHandler` 滿 300 折 30 與 `totalPrice` 扣減)
- `src/frontend/templates/cart.html:88-100` (購物車 Discount 列與 Total 列 HTML 結構)
- `src/checkoutservice/main.go:229-283` (`PlaceOrder` 訂單扣款計算與 `OrderResult` 組裝)
- `src/checkoutservice/main.go:242-257` (`PlaceOrder` 扣除折扣後向 `paymentservice.Charge` 請款)
- `src/checkoutservice/main.go:267-274` (`PlaceOrder` 指派 `OrderResult.Discount`)
- `src/checkoutservice/main.go:292-334` (`prepareOrderItemsAndShippingQuoteFromCart` 門檻判定與折後運費詢價)
- `protos/demo.proto:209-216` (`OrderResult` Protobuf 定義，包含欄位 6 `discount`)
- `src/checkoutservice/genproto/demo.pb.go:1409-1420` (`OrderResult` Go 結構定義)
- `src/checkoutservice/genproto/demo.pb.go:1488-1493` (`OrderResult.GetDiscount` 方法定義)
- `src/checkoutservice/genproto/demo.pb.go:1831-2130` (`file_demo_proto_rawDesc` Protobuf 二進位描述子)
- `src/checkoutservice/genproto/demo.pb.go:2160-2210` (`file_demo_proto_depIdxs` 描述子型態索引)
- `src/frontend/genproto/demo.pb.go:1409-1420` (`frontend` 端 `OrderResult` Go 結構定義)
- `src/frontend/genproto/demo.pb.go:1831-2130` (`frontend` 端 `file_demo_proto_rawDesc` 描述子)
- `src/frontend/handlers.go:380-430` (`placeOrderHandler` 結帳送出與 `totalPaid` 計算)
- `src/frontend/handlers.go:406-414` (`placeOrderHandler` 加總品項與 `GetDiscount()` 判定)
- `src/frontend/templates/order.html:56-73` (訂單完成頁 Discount 條件區塊與 Total Paid 結構)
