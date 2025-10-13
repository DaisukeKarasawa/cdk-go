---
allowed-tools: Bash(git status:*), Bash(git diff:*), Bash(git log:*), Bash(git add:*), Bash(git commit:*), Bash(gh pr view:*), Bash(gh pr comment:*), Bash(gh api:*)
description: CodeRabbitのレビューコメントを読み取り、修正の妥当性を判断して修正を適用し、各ディスカッションに個別リプライで報告する
argument-hint: [pr-number]
---

# レビューフィードバック対応コマンド

CodeRabbitのレビューコメントを読み取り、修正の妥当性を深く考え、必要に応じて修正を適用し、**各ディスカッションに個別リプライ**で報告します。

## 現在の状況

### Git ステータス

!`git status --porcelain`

### 現在のブランチ

!`git branch --show-current`

### 指定されたPRの詳細

!`if [ -n "$ARGUMENTS" ]; then gh pr view $ARGUMENTS; else echo "PR番号が指定されていません。現在のブランチのPRを自動検出します"; fi`

### PRのコメント一覧

!`if [ -n "$ARGUMENTS" ]; then gh pr view $ARGUMENTS --comments; else gh pr list --head $(git branch --show-current) --json number --jq '.[0].number' | xargs -I {} gh pr view {} --comments; fi`

### PRのディスカッション（レビューコメント）

!`if [ -n "$ARGUMENTS" ]; then gh api repos/:owner/:repo/pulls/$ARGUMENTS/comments; else gh pr list --head $(git branch --show-current) --json number --jq '.[0].number' | xargs -I {} gh api repos/:owner/:repo/pulls/{}/comments; fi`

## あなたのタスク

上記の情報を基に、以下の手順でレビューフィードバックに対応してください：

### 1. **コメントとディスカッションの読み取り**

- PRのコメント一覧を確認
- CodeRabbitからのレビューコメントを特定
- ディスカッション（具体的なコードレビュー）を確認
- 「Prompt for AI Agents」を含むコメントを特定
- 各コメントのIDを記録（リプライに必要）

### 2. **修正の妥当性の深い検討**

各レビューコメントについて以下を検討：

- **技術的正確性**: 指摘された問題は実際に存在するか？
- **影響範囲**: 修正による影響は適切か？
- **設計思想**: 現在の設計意図と矛盾しないか？
- **優先度**: Critical、Major、Minorの分類は適切か？
- **代替案**: より良い解決策はないか？

### 3. **修正の適用**

#### 3.1. **Prompt for AI Agentsがある場合**

- プロンプトの内容を正確に理解
- プロンプトに従って修正を適用
- プロンプトの意図を尊重

#### 3.2. **その他の修正**

- 技術的に妥当と判断された修正のみ適用
- 影響範囲を最小限に抑制
- 既存のテストが壊れないことを確認

### 4. **非ソースコードファイルの修正**

修正に伴い更新が必要なファイル：

- **ドキュメント**: README、設計書、API仕様書
- **設定ファイル**: go.mod、Dockerfile、Makefile
- **テストファイル**: テストケースの追加・修正
- **CI/CD**: GitHub Actions、デプロイスクリプト

### 5. **各ディスカッションに個別リプライ**

#### 5.1. **修正した箇所**

各CodeRabbitのコメントに対して個別にリプライ：

**手順：**

1. 各コメントIDを特定
2. JSONファイルを作成してリプライ内容を準備
3. GitHub APIでディスカッションにリプライ
4. 一時ファイルを削除

**実装例：**

````bash
# 1. リプライ内容をJSONファイルに保存
cat > /tmp/reply_${comment_id}.json << 'EOF'
{
  "body": "✅ **修正完了**\n\n[具体的な修正内容]\n\n```diff\n- 修正前のコード\n+ 修正後のコード\n```\n\n[修正理由と影響範囲の説明]"
}
EOF

# 2. ディスカッションにリプライ
gh api repos/:owner/:repo/pulls/${PR_NUMBER}/comments/${comment_id}/replies \
  --method POST \
  --input /tmp/reply_${comment_id}.json

# 3. 一時ファイルを削除
rm /tmp/reply_${comment_id}.json
````

**リプライ内容のテンプレート：**

````markdown
✅ **修正完了**

[修正内容の説明]

```diff
- 修正前のコード
+ 修正後のコード
```
````

[修正理由と影響範囲の説明]

````

#### 5.2. **修正しなかった箇所**

修正を見送った場合のリプライテンプレート：

```markdown
🤔 **修正を見送りました**

**見送り理由**: [修正しない判断の根拠]

**技術的判断**: [設計思想との整合性、影響範囲の考慮]

**代替案**: [将来的な改善案があれば]
```

#### 5.3. **リプライ後の確認**

- 各ディスカッションが個別のスレッドとして表示されることを確認
- 必要に応じて「Resolve conversation」で解決済みにマーク
- 未解決のディスカッションのみが表示されることを確認

### 6. **全体サマリーコメント（オプション）**

すべての個別リプライ完了後、必要に応じて全体のサマリーを投稿：

```bash
gh pr comment ${PR_NUMBER} --body "## 📋 レビューフィードバック対応完了

すべてのCodeRabbitのディスカッションに個別に対応しました。

### 修正完了項目
- [修正した項目のリスト]

### 見送り項目
- [見送った項目のリスト]

各ディスカッションは個別に「Resolve conversation」で解決できます。"
```

## 実行例

```bash
# 特定のPRのレビューフィードバックに対応
/review-feedback 1

# 現在のブランチのPRのレビューフィードバックに対応
/review-feedback
```

## 修正の判断基準

### ✅ **修正すべき項目**

- **Critical**: セキュリティ問題、データ損失の可能性
- **Major**: 機能障害、パフォーマンス問題
- **Minor**: コード品質、保守性の向上（影響範囲が小さい場合）

### ❌ **修正を見送る項目**

- 設計思想と矛盾する提案
- 影響範囲が大きすぎる修正
- 現在の要件に不要な機能追加
- テストカバレッジが不十分な修正

## エラーハンドリング

- **PRが存在しない場合**: エラーメッセージを表示
- **レビューコメントがない場合**: 確認メッセージを表示
- **修正に失敗した場合**: エラー内容をディスカッションにリプライで報告
- **コミットに失敗した場合**: 詳細なエラー情報を提供
- **APIエラーの場合**: リトライまたは手動操作の案内

## 注意事項

- 修正前に必ず現在のコードをバックアップ
- 修正後は必ずテストを実行
- コミットメッセージは修正内容を明確に記述
- **各ディスカッションに個別リプライ**することで、レビューの進行状況を明確化
- **JSONファイルを使用**してシェル構文エラーを回避
- **一時ファイルは必ず削除**してクリーンアップ
- 引数として提供されたPR番号を使用：$ARGUMENTS

## 成功時の出力

修正完了後に以下を表示：

- 修正したファイル数と内容
- 各ディスカッションへのリプライURL
- 解決済みにできるディスカッション数
- 次のステップの提案（Resolve conversationの案内）

## 技術的詳細

### GitHub API エンドポイント

```bash
# ディスカッションにリプライ
POST /repos/{owner}/{repo}/pulls/{pull_number}/comments/{comment_id}/replies

# 使用例
gh api repos/owner/repo/pulls/1/comments/123456789/replies \
  --method POST \
  --input reply.json
```

### JSONファイル形式

```json
{
  "body": "リプライ内容（Markdown形式）"
}
```

--- End Command ---
