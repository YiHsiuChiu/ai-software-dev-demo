## Solution Proposals

### Context
- Request: 開發新功能，在結帳時訂單金額滿 300 時折扣 30 元，並標明折扣了多少錢；此折扣計算必須在計算免運費（滿 200 免運費）之前執行。
- Research Source: `research/spend300get30off/research.md`（第 1 節 gRPC 協定定義、第 2 節 運費計算與免運費實作、第 3 節 結帳編排服務實作、第 4 節 前端展示與結帳流程實作、第 5 節 金額運算模組現況、第 6 節 週邊服務現況與 Data Flow 章節）。

---

### Proposal 1 — 擴充 Proto 契約並集中由結帳服務主導折扣（Contract-Driven Proposal）

- Overview:
  本方案修改 `protos/demo.proto`，在 `OrderResult` 中新增折扣欄位 `Money discount`，由後端 `checkoutservice` 作為訂單與促銷金額的單一事實來源（Single Source of Truth）。`checkoutservice` 在結帳時判定商品 USD 小計是否滿 300 USD，扣除 30 USD 折扣後將折後金額傳遞給 `shippingservice.GetQuote` 作為免運判定依據，並扣除信用卡款後將包含折扣之 `OrderResult` 回傳給前端展示與記錄。

- Key Changes:
  - `protos/demo.proto`：
    - 在 `OrderResult`（`protos/demo.proto:209-215`）中新增欄位：`Money discount = 6;`。
    - 重新執行 `checkoutservice` 與 `frontend`（可選包含 `emailservice`）的 `genproto.sh` 產生最新 gRPC stubs。
  - `src/checkoutservice/`：
    - 在 `src/checkoutservice/money/money.go:1-132` 擴充金額扣減（`Sub`）與比較（`Compare` 或 `GreaterThanOrEqual`）公用函式。
    - 在 `src/checkoutservice/main.go:287-310`（`prepareOrderItemsAndShippingQuoteFromCart`）中，於 `prepOrderItems` 取得 `itemsSubtotalUSD` 後，若 $\ge$ 300 USD，計算 30 USD 折扣，並以折後美金小計（`itemsSubtotalUSD - 30 USD`）作為 `ItemsSubtotalUsd` 傳入 `quoteShipping`（`src/checkoutservice/main.go:312-322`）供 `shippingservice.GetQuote` 進行滿 200 免運判定。
    - 將 30 USD 折扣以 `currencyservice.Convert` 轉為使用者幣別，在 `src/checkoutservice/main.go:242-255` 計算總扣款額 `total = itemsTotalLocalized - discountLocalized + shippingCostLocalized` 並送交 `paymentservice.Charge`。
    - 將 `discountLocalized` 存入 `OrderResult.Discount`（`src/checkoutservice/main.go:264-270`）。
  - `src/frontend/`：
    - 在 `src/frontend/money/money.go:1-132` 同步加入 `Sub` 與 `Compare` 函式。
    - 在 `src/frontend/handlers.go:251-324`（`viewCartHandler`）中，於計算 `itemsUSD` 後若達 300 USD，以換匯後的 30 USD 作為折扣，將折後美金小計傳給 `fe.getShippingQuote`（`src/frontend/rpc.go:87-98`），並將折扣金額 `discount` 注入模板資料。
    - 修改 `src/frontend/templates/cart.html:88-96`，新增 Discount 列顯示折扣金額（如 `- $30.00`）與折後總額。
    - 在 `src/frontend/handlers.go:326-407`（`placeOrderHandler`）中，直接自 `order.GetOrder().GetDiscount()` 讀取折扣金額，計算實付總額 `totalPaid`，並於 `src/frontend/templates/order.html:40-63` 顯示折扣明細與實付總額。

- Trade-offs:
  - 優點：
    1. **單一資料來源與架構清晰**：訂單實體（`OrderResult`）包含完整的折扣欄位，前端下單完成後直接讀取後端確認的折扣資訊，避免前後端自行推算產生的邏輯脫節。
    2. **系統擴充性佳**：未來若其他服務（例如發送通知的 `emailservice`、報表分析或發票系統）需要折扣資訊，可直接自 Proto 結構讀取，無須重新設計契約。
  - 缺點與風險：
    1. **Proto 變更成本**：修改 `protos/demo.proto` 需重新產生多個服務的 protobuf Go 程式碼，增加建置與維護步驟。
    2. **異動服務較多**：改動涵蓋 Proto 定義層、`checkoutservice` 與 `frontend`。

- Validation:
  - 單元測試：
    - 於 `src/checkoutservice/money/money_test.go` 與 `src/frontend/money/money_test.go` 新增 `Sub` 與 `Compare` 函式之單元測試。
    - 針對 `checkoutservice` 與 `frontend` 的金額折抵計算加入測試案例，驗證：
      - 未滿 300 USD（如 250 USD）：折扣為 0，折後 250 USD $\ge$ 200 USD 享有免運。
      - 剛好 300 USD：折扣 30 USD，折後 270 USD $\ge$ 200 USD 享有免運。
      - 超過 300 USD（如 350 USD）：折扣 30 USD，折後 320 USD $\ge$ 200 USD 享有免運。
  - 整合驗證：
    - 透過 `docker compose up --build -d` 重新建置啟動服務。
    - 購物車介面驗證：加入滿 $300 商品，檢查 `/cart` 頁面顯示 Discount `- $30.00`、Shipping `$0.00` 以及折抵後正確的 Total。
    - 結帳扣款驗證：送出結帳表單，檢查完成頁 `/order` 顯示折扣與實付總額，並核對 `paymentservice` 容器日誌中實際請款金額確實扣減 $30。

- Open Questions:
  - 非 USD 幣別時的計算標準：商品原價均為 USD（`src/productcatalogservice/products.json:1-112`）。門檻判斷是否固定以商品原始 USD 小計滿 300 判定折 30 USD，再依當日匯率轉換為使用者幣別？
  - `emailservice`（`src/emailservice/templates/confirmation.html:31-51`）是否需要同步在此階段擴充顯示折扣欄位？

---

### Proposal 2 — 維持既有 Proto 契約，於應用層實作折扣計算與展示（Application-Level Proposal）

- Overview:
  本方案維持 `protos/demo.proto` 完全不變，不更動任何 gRPC 通訊協定。滿 300 折 30 之門檻判定、折抵後小計與免運先行扣除規則，分別在 `checkoutservice`（負責扣款計算）與 `frontend`（負責購物車展示與下單結果呈現）的 Go 應用程式碼中獨立實作。

- Key Changes:
  - `protos/demo.proto`：
    - **完全不修改**，不更動現有介面，無須重新產生 protobuf 程式碼。
  - `src/checkoutservice/`：
    - 在 `src/checkoutservice/money/money.go:1-132` 擴充 `Sub` 與 `Compare` 函式。
    - 在 `src/checkoutservice/main.go:287-310` 中，取得 `itemsSubtotalUSD` 後判定若 $\ge$ 300 USD，產生 30 USD 折扣；以折後小計傳給 `quoteShipping`，向 `shippingservice.GetQuote` 取得免運運費。
    - 在 `src/checkoutservice/main.go:242-255` 計算最終請款金額：`total = itemsTotal - discount + shippingCost`，將正確折抵後金額送交 `paymentservice.Charge`。
  - `src/frontend/`：
    - 在 `src/frontend/money/money.go:1-132` 擴充 `Sub` 與 `Compare` 函式。
    - 在 `src/frontend/handlers.go:251-324`（`viewCartHandler`）中，若 `itemsUSD` $\ge$ 300 USD，計算折抵金額（30 USD 換匯為目前幣別），以折後美金小計傳給 `fe.getShippingQuote`，並將 `discount` 變數注入模板以在 `src/frontend/templates/cart.html:88-96` 渲染折扣行與折後金額。
    - 在 `src/frontend/handlers.go:326-407`（`placeOrderHandler`）中，因為 `OrderResult` 無折扣欄位，前端依據品項總計是否達 300 USD 門檻，自行扣減折抵金額以計算出與後端一致之 `totalPaid`，並傳入 `src/frontend/templates/order.html:40-63` 渲染。

- Trade-offs:
  - 優點：
    1. **完全零 Proto 變更風險**：無需修改 `protos/demo.proto` 與執行 `genproto.sh`，完全規避微服務間 protobuf 產生代碼版本不一致的編譯或相容性風險。
    2. **改動影響範圍最小**：所有變動集中在 `checkoutservice` 與 `frontend` 的 Go 業務邏輯與 HTML 模板，修改單純且聚焦。
  - 缺點與風險：
    1. **業務規則重複存在（Duplication of Logic）**：滿 300 折 30 的促銷邏輯必須分別寫在 `frontend`（展示與訂單完成總額）與 `checkoutservice`（扣款），存在兩端實作不一致的維護風險。
    2. **訂單實體語意缺失**：`OrderResult` 沒有紀錄折扣資訊，若未來有其他依賴 `OrderResult` 的服務（如 `emailservice`），將無法取得折扣歷史紀錄。

- Validation:
  - 單元測試：
    - 於 `checkoutservice/money` 與 `frontend/money` 執行 `go test ./...` 驗證金額運算正確性。
  - 整合驗證：
    - 啟動 Docker Compose，在前端分別測試購物車金額在 $299.99 與 $300.00 以上時的購物車顯示與結帳扣款金額，確認前端顯示之 `totalPaid` 與 `checkoutservice` 傳給 `paymentservice` 的金額完全一致。

- Open Questions:
  - 未來若促銷規則調整（例如門檻改變或新增其他折扣），兩端同步維護之負擔是否可接受？
