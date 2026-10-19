根據 Research Document、使用者需求，以及 Propose 階段選定的方案，撰寫具體且可執行的 Implementation Plan。

## 執行步驟（Steps）

1. **確認修改範圍**：確認功能需求、修改範圍、限制條件，以及使用者指定的時程（如有）。若尚未選定方案，先詢問使用者，不要自行決定。
2. **閱讀 Design Document Template**：完整閱讀 `@design_doc_template.md`，依照其結構，在 `desired_location` 建立 Markdown 格式的 Design Document。
3. **撰寫實作計畫**：根據現有程式碼與選定方案，簡潔且明確地記錄：
   - Current Context 與 Requirements。
   - Design Decisions 及選擇該方案的理由。
   - Technical Design，包括相關 Components、Data Flow、API，以及預計修改的檔案與行號。
   - Implementation Plan，將開發工作拆解為可以依序執行的 Tasks。
   - Testing Strategy，列出必要的測試案例與驗證方式。
4. **限制修改範圍**：在 Files Changed 中明確列出預計新增或修改的檔案。除了經使用者同意的例外，後續 Implement 階段只能修改這些檔案。
5. **保持設計精簡**：只規劃本次需求直接需要的變更，不加入額外功能、不必要的重構或過度設計。
6. **不得實作**：此階段只能建立或更新 Design Document，不得修改任何程式碼、測試或系統設定。完成後等待使用者確認，才能進入 Implement 階段。