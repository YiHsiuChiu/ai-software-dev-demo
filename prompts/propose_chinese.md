根據既有的 Research Document，以及使用者提出的新功能或專案需求，產生最多兩個不同的解決方案（Solution Proposals）。

## 行為規範（Behavior Contract）

- 將提供的 `research-codebase` Markdown 文件作為研究背景與文件結構的主要參考依據。
- 必須先取得由 `/research-codebase` 產生的 Research Document，才能開始提出方案。
- 根據使用者需求，提出最多兩種不同的實作方案，每個方案都必須有 Research Findings 作為依據。
- 每個方案都需要說明 Trade-offs、受影響的系統、驗證方式與尚未釐清的問題。
- 此階段只提出方案，不修改程式碼。

## 執行步驟（Steps）

### 1. 接收需求（Intake）

- 確認使用者提出的功能或專案需求。
- 取得 Research Document 的檔案路徑（如果使用者有提供）。

### 2. 解析研究文件（Parse Research）

- 完整閱讀 Research Document。
- 從文件中整理出與需求相關的限制條件、Modules、Dependencies、Data Flow，以及既有的設計決策。

### 3. 整理解決方案（Synthesize Solution Space）

- 根據 Research Findings，提出最多兩個符合現有架構、且有明確差異的方案。
- 每個方案簡要說明：
  - 核心實作方式。
  - 預計修改的 Components、Modules 或程式碼位置。
  - 主要優缺點與 Trade-offs。
- 依照與需求及現有架構的符合程度排序。

### 4. 規劃驗證方式（Validation Planning）

- 簡要說明如何透過現有測試或必要的新測試，驗證各方案是否符合需求。
- 列出會影響方案選擇、但目前尚未確認的重要問題；沒有則省略。

### 5. 產出方案文件

依照以下格式輸出 Solution Proposals。

## 輸出格式（Output Template）

```markdown
## Solution Proposals

### Context
- Request: <簡述使用者需求>
- Research Source: <研究文件名稱與引用章節>

### Proposal 1 — <方案名稱>
- Overview: <2–3 句說明方案>
- Key Changes: <主要修改的 Components / Modules>
- Trade-offs: <優點、缺點與風險>
- Validation: <測試方式或驗證方法>
- Open Questions: <尚未釐清的問題>

### Proposal 2 — <方案名稱>
- Overview: <2–3 句說明方案>
- Key Changes: <主要修改的 Components / Modules>
- Trade-offs: <優點、缺點與風險>
- Validation: <測試方式或驗證方法>
- Open Questions: <尚未釐清的問題>
```

## 注意事項（Notes）

- 引用 Research Findings 中的具體程式碼時，使用 `path/to/file.py:123-145` 格式標示檔案與行號。
- 如果 Research Document 沒有涵蓋需求所需的重要資訊，必須明確指出資訊缺口，並建議先補充 Research，而不是自行假設。
- 保持內容精簡、清楚且可執行，避免不必要的細節。
- **不要在此階段實作功能或修改程式碼。**