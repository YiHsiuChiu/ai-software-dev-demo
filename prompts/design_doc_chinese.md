# [FEATURE] Design Document

## 1. Current Context

- 簡述與本次功能相關的現有系統架構。
- 說明相關 Components、Services 及其關係。
- 描述本次需求要解決的問題。

## 2. Requirements

### Functional Requirements

- 列出本次功能必須達成的行為。
- 說明重要的業務規則與邊界條件。
- 列出需要維持的既有行為。

### Non-Functional Requirements

- 僅列出與本次功能直接相關的效能、相容性、安全性或其他限制。
- 沒有額外要求時可省略。

## 3. Design Decisions

### Selected Approach

- **選定方案：** <Propose 階段核准的方案>
- **核心設計：** <簡述實作方式>
- **選擇原因：** <為什麼採用此方案>
- **Trade-offs：** <重要的優缺點或限制>

## 4. Technical Design

### Core Components

- 列出需要修改的 Components / Services。
- 說明各 Component 的責任與預計調整的邏輯。

### Data Flow & Integration

- 說明功能相關的 Data Flow。
- 若涉及跨服務呼叫，說明相關 API Contract 與資料傳遞方式。
- 只有必要時才加入介面定義或示意圖。

### Files Changed

明確列出本次預計新增或修改的所有檔案。

每個既有檔案都必須提供實際行號，格式如下：

- `src/service/file.go:23-49` — <修改內容>
- `src/service/handler.go:80-120` — <修改內容>
- `src/service/handler_test.go:15-60` — <新增或修改的測試>

新檔案請標記 `(new file)`。

不得猜測行號。若無法確認，必須明確註記。

**Implement 階段應以此清單作為修改範圍，未經確認不得擴大。**

## 5. Implementation Plan

按照執行順序列出具體 Tasks。

### Step 1 — <任務名稱>
- <需要修改的檔案或 Component>
- <具體修改內容>
- <完成條件>

### Step 2 — <任務名稱>
- <需要修改的檔案或 Component>
- <具體修改內容>
- <完成條件>

### Step 3 — <任務名稱>
- <需要修改的檔案或 Component>
- <具體修改內容>
- <完成條件>

## 6. Testing Strategy

只規劃對本次變更有價值的測試，避免過度 Mock 或建立不必要的測試。

### Unit Tests

- 列出需要實作的具體測試案例與預期結果。
- 如果不需要新增 Unit Tests，簡述原因。

### Integration / Functional Tests

- 列出需要驗證的跨 Component 行為或使用者流程。
- 說明必要的測試資料與預期結果。
- 使用專案現有的測試工具與 Docker Compose 環境。

## 7. References

- Research Document
- 選定的 Solution Proposal
- 其他本次設計實際參考的文件（如有）