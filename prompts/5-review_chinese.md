Review 目前 Branch 中所有尚未 Commit 的變更（包含 Staged、Unstaged 和新增檔案），並產出依重要程度排序的修改建議。

## 行為規範（Behavior Contract）

- 以已核准的 Design Document 和功能需求為依據，確認實作是否符合預期。
- 優先檢查功能正確性、潛在 Bug、Edge Cases、Regression 和跨服務整合風險。
- 只有在與本次變更直接相關時，才檢查 Security、Performance 或 Dependencies。
- 避免提出不必要的重構、命名或程式碼風格建議。
- 此階段只進行 Review，不修改任何程式碼。

## 執行步驟（Steps）

### 1. 收集變更資訊

執行以下指令：

```bash
git status --porcelain
git diff
git diff --cached
git diff HEAD
```

- 檢查 Staged、Unstaged，以及尚未被 Git 追蹤的新增檔案。
- 閱讀相關的 Design Document 和功能需求。
- 必要時查看相關的既有程式碼，了解變更對其他 Components 的影響。

### 2. 分析程式碼變更

重點檢查：

- **Correctness**：功能是否符合需求及 Design Document。
- **Edge Cases**：是否遺漏重要的邊界條件或錯誤處理。
- **Regression**：是否破壞原有功能。
- **Integration**：跨 Component / Service 的資料流與 API 是否一致。
- **Testing**：是否缺少對本次功能有價值的測試。

只回報有具體依據、可能影響程式正確性或可維護性的問題。

### 3. 產出 Review 結果

提供簡短摘要，以及依優先級排序的單一 Action Items 清單。

優先級定義：

- 🔴 **Must Fix**：明確的 Bug、需求不符，或可能造成重大 Regression 的問題。
- 🟡 **Recommended**：有實際風險或重要測試缺口，建議修正。
- 🟢 **Consider**：非必要，但有明確價值的小幅改善。

## 輸出格式（Output Template）

```markdown
## Code Review

### Summary
<1–2 句總結實作狀況及主要風險>

### Action Items

1. 🔴 Must Fix — <問題與建議修正方式>
   - Location: `path/to/file.go:23-49`
   - Reason: <為什麼這是問題>

2. 🟡 Recommended — <問題與建議修正方式>
   - Location: `path/to/file.go:80-95`
   - Reason: <可能造成的影響>

3. 🟢 Consider — <改善建議>
   - Location: `path/to/file.go:120`
   - Reason: <改善價值>
```

## 注意事項（Notes）

- 每個 Action Item 都必須提供具體的檔案路徑與行號。
- 只列出實際發現的問題，不需要為了填滿優先級而產生建議。
- 若沒有發現需要修正的問題，明確說明即可。
- 不要重複列出相同問題。
- 不要修改程式碼，等待使用者確認 Review 結果。