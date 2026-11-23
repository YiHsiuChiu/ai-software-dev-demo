# [FEATURE] Design Document: 結帳完成頁面金額與折扣顯示一致性修正 (Order Amount Discrepancy Fix)

## 1. Current Context

目前 Online Boutique 系統在滿 300 折 30 的功能實作中：
- 購物車檢視（`GET /cart`）由 `frontendServer.viewCartHandler` 直接在本地計算商品小計並扣除 30 USD 折扣，故購物車頁面預估總額顯示折抵後的 299.97 USD。
- 結帳流程（`POST /cart/checkout`）由 `checkoutservice.PlaceOrder` 計算 30 USD 折扣並向 `paymentservice` 請款 299.97 USD（信用卡實際扣款金額正確）。
- 然而在訂單送出後的回傳流程中，由於 gRPC 自動生成檔案 `demo.pb.go` 之底層二進位描述子未註冊 `discount` 欄位索引，導致前端接收到的 `order.GetOrder().GetDiscount()` 為 `nil`。
- 前端 `placeOrderHandler` 以品項加總計算 `totalPaid`，在未能讀取到 `order.Discount` 時未執行折扣扣減，致使訂單完成頁面（`/order`）顯示未折抵之 329.97 USD，且未渲染折扣明細。

本次需求旨在修正此顯示不一致問題，確保購買 3 支 Watch 時，訂單完成頁面之折扣明細（`- $30.00`）與實付總額（`Total Paid: $299.97`）能與購物車預估金額完全一致。

---

## 2. Requirements

### Functional Requirements

1. **結帳完成頁面實付金額（Total Paid）一致性**：
   - 當訂單商品總額滿 300 USD 時，訂單完成頁面顯示的 `Total Paid` 必須正確扣減 30 USD（換匯為當前幣別）。
   - 購買 3 支單價 109.99 USD 的手錶時，`Total Paid` 必須正確顯示為 299.97 USD。
2. **折扣明細顯示（Discount Row）**：
   - 訂單完成頁面必須顯示折扣明細列：`Discount: - $30.00`（或對應幣別金額）。
   - 未滿 300 USD 的訂單則不顯示折扣明細列。
3. **維持向後相容性**：
   - 若未來後端 gRPC 介面修復並回傳非空的 `order.Discount`，前端能相容接收與顯示。

### Non-Functional Requirements

- **零 Protoc 工具鏈依賴**：改動完全集中於前端應用層，不依賴本機或外部容器之 `protoc` 編譯器重新生成 protobuf 程式碼。
- **變更最小化與局部收斂**：僅修改前端處理常式與 HTML 模板，不影響其他後端微服務的運行。

---

## 3. Design Decisions

### Selected Approach

- **選定方案：** Proposal 1 — 前端直接根據訂單品項與門檻計算折扣並展示（Frontend Autonomous Calculation）。
- **核心設計：**
  1. 在 `src/frontend/handlers.go` 的 `placeOrderHandler` 中，延續前端自行走訪 `order.GetOrder().GetItems()` 計算金額的既有架構模式，於加總時一併計算訂單商品的美金小計 `itemsUSD`。
  2. 判定 `itemsUSD` 是否 $\ge$ 300 USD；若成立，透過 `fe.convertCurrency` 計算換匯後之折扣金額 `discountCost`，並自 `totalPaid` 中扣減。
  3. 將計算出的 `discountCost` 作為獨立資料欄位 `"discount"` 注入 `order.html` 模板。
  4. 在 `src/frontend/templates/order.html` 中優先讀取傳入之 `.discount`（並維持對 `.order.Discount` 的回退相容），正確顯示折扣明細列與扣減後之實付總額。
- **選擇原因：**
  - **即時且高可靠度**：徹底解耦對底層 Protobuf runtime 描述子二進位註冊的相依性，避免跨平台環境（如 Windows 本機缺少 protoc）造成的反序列化丟失問題。
  - **改動影響範圍最小**：無需變更 Proto 契約或重新建置後端微服務，僅更新前端即可迅速解決顯示差額問題。
- **Trade-offs：**
  - *優點*：修正速度快、修改行數少、穩定性高、完全相容既有架構。
  - *缺點*：滿 300 折 30 的促銷邏輯在前端 `placeOrderHandler` 與 `viewCartHandler` 各自存在計算步驟。

---

## 4. Technical Design

### Core Components

1. **`src/frontend/handlers.go` (`placeOrderHandler`)**：
   - 負責在訂單下單成功後計算顯示用的已付總額 `totalPaid`。
   - 調整邏輯：在累加 `multPrice` 時，透過 `fe.getProduct` 累加商品 USD 原價至 `itemsUSD`；若 $\ge$ 300 USD，產生 30 USD 之本地貨幣折扣，自 `totalPaid` 扣減，並傳入模板 context。
2. **`src/frontend/templates/order.html`**：
   - 調整 Discount 列的條件渲染：支援讀取傳入之 `.discount`，顯示 `- {{ renderMoney .discount }}`。

### Data Flow & Integration

```
[Browser]
    │  POST /cart/checkout
    ▼
[frontend: placeOrderHandler] (src/frontend/handlers.go:326)
    │
    ├─► checkoutservice.PlaceOrder (後端已正確扣款 $299.97)
    │     └─► 回傳 OrderResult
    │
    ├─► 走訪 order.GetOrder().GetItems():
    │     - multPrice 累加至 totalPaid ($329.97)
    │     - product.PriceUsd 累加至 itemsUSD ($329.97)
    │
    ├─► 判定 itemsUSD >= $300:
    │     - discountUSD = $30.00
    │     - discountCost = convertCurrency($30.00, userCurrency)
    │     - totalPaid = totalPaid - discountCost ($299.97)
    │
    ▼
[HTML Response: order.html] (src/frontend/templates/order.html)
    - 渲染 Discount: - $30.00
    - 渲染 Total Paid: $299.97
```

### Files Changed

- `src/frontend/handlers.go:406-430` — 在 `placeOrderHandler` 累加計算品項 USD 小計、判定滿 300 折 30、扣除 `totalPaid` 並注入 `"discount": discountCost` 至模板資料
- `src/frontend/templates/order.html:56-65` — 擴充折扣明細條件判斷，支援讀取 `.discount` 並顯示折扣金額

---

## 5. Implementation Plan

### Step 1 — 調整前端下單處理常式 (`placeOrderHandler`)
- **修改檔案**：`src/frontend/handlers.go:406-430`
- **具體修改內容**：
  - 在走訪 `order.GetOrder().GetItems()` 迴圈中，呼叫 `fe.getProduct` 取得商品美金單價並累加至 `itemsUSD`。
  - 比對 `itemsUSD` 是否達到 300 USD 門檻：
    - 若滿額，以 `fe.convertCurrency` 將 30 USD 換匯為使用者幣別，並自 `totalPaid` 扣減。
    - 若 gRPC `order.GetOrder().GetDiscount()` 非空亦可相容扣除。
  - 在 `templates.ExecuteTemplate(w, "order", ...)` 中注入 `"discount": discountCost`。
- **完成條件**：
  - `totalPaid` 變數正確計算為 299.97 USD，且模板 context 包含 `discount` 欄位。

### Step 2 — 調整訂單完成 HTML 模板 (`order.html`)
- **修改檔案**：`src/frontend/templates/order.html:56-65`
- **具體修改內容**：
  - 調整 Discount 區塊條件式，若傳入之 `.discount` 存在且大於 0，渲染折扣明細行：`- {{ renderMoney .discount }}`。
- **完成條件**：
  - 訂單完成頁面能正確顯示 Discount 項目。

### Step 3 — 驗證與回歸檢查
- **修改檔案**：無（執行驗證）
- **具體修改內容**：
  - 重建前端容器：`docker compose up --build -d frontend`。
  - 測試加入 3 支手錶（$329.97）：
    - 購物車 `/cart` 預估金額為 $299.97。
    - 結帳後完成頁 `/order` 顯示 `Discount: - $30.00`，`Total Paid: $299.97`。
  - 測試加入 1 支手錶（$109.99）：
    - 購物車 `/cart` 與完成頁 `/order` 皆顯示 $109.99，無折扣行。
- **完成條件**：
  - 購物車預估金額與結帳完成頁實付金額完全一致。

---

## 6. Testing Strategy

### Unit Tests
- 本次僅更動前端展示資料組裝與模板，既有 `src/frontend/money/money_test.go` 與 `validator_test.go` 保持綠燈。

### Integration / Functional Tests
1. **滿額折抵一致性驗證**：
   - 瀏覽器開啟 `http://localhost:8080`，將 3 支 Watch 加入購物車。
   - 驗證 `/cart` 總額為 $299.97，折扣顯示 `- $30.00`。
   - 點擊送出結帳，驗證 `/order` 頁面顯示：
     - `Discount: - $30.00`
     - `Total Paid: $299.97`
2. **未達門檻回歸驗證**：
   - 加入單價低於 $300 商品結帳，驗證 `/order` 不顯示 Discount，`Total Paid` 等於各品項與運費之原價加總。

---

## 7. References

- [order_amount_discrepancy_research.md](file:///c:/Users/NCYU/Desktop/陳允瀚/軟工實務助教/115學年/教學計劃/ai-software-dev-demo/research/spend300get30off/order_amount_discrepancy_research.md)
- [order_amount_discrepancy_proposals.md](file:///c:/Users/NCYU/Desktop/陳允瀚/軟工實務助教/115學年/教學計劃/ai-software-dev-demo/research/spend300get30off/order_amount_discrepancy_proposals.md) (Proposal 1)
- [design_doc_template.md](file:///c:/Users/NCYU/Desktop/陳允瀚/軟工實務助教/115學年/教學計劃/ai-software-dev-demo/prompts/design_doc_template.md)
