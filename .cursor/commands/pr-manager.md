---
allowed-tools: Bash(git status:*), Bash(git diff:*), Bash(git log:*), Bash(git push:*), Bash(gh pr create:*), Bash(gh pr list:*), Bash(gh pr view:*), Bash(gh pr edit:*)
description: 現在のブランチの状況を判定してPRの作成または更新を自動実行する
argument-hint: [base-branch-or-pr-number]
---

# PR管理統合コマンド

現在のブランチの状況を自動判定して、PRの作成または既存PRの更新を実行します。

## 現在の状況

### Git ステータス

!`git status --porcelain`

### 現在のブランチ

!`git branch --show-current`

### 現在のブランチのPR一覧

!`gh pr list --head $(git branch --show-current)`

### リモートブランチの存在確認

!`git ls-remote --heads origin $(git branch --show-current) | wc -l`

### コミット履歴（現在のブランチ vs master）

!`git log --oneline master..HEAD --format="%h %s"`

### 変更差分の統計

!`git diff master..HEAD --stat`

### 変更されたファイルの詳細

!`git diff master..HEAD --name-status`

## あなたのタスク

上記の情報を基に、以下の手順でPRを管理してください：

### 1. **状況の判定**

以下の条件に基づいて適切なアクションを決定：

- **既存PRがある場合** → PR更新モード
- **既存PRがない場合** → PR作成モード
- **リモートブランチがない場合** → プッシュしてからPR作成
- **変更がない場合** → 警告表示

### 2. **PR作成モード（既存PRがない場合）**

1. **リモートプッシュ**（必要な場合）：

   - リモートブランチが存在しない場合は先にプッシュ
   - `git push -u origin $(git branch --show-current)`

2. **変更内容の分析**：

   - 変更されたファイルの種類と内容を確認
   - 変更の性質（新機能、バグ修正、リファクタリングなど）を判断

3. **適切なタイトルの生成**：

   - Conventional Commitsの形式に従う
   - `feat:` - 新機能の追加
   - `fix:` - バグ修正
   - `docs:` - ドキュメントの変更
   - `refactor:` - コードのリファクタリング
   - その他の適切なプレフィックス

4. **詳細な説明の作成**：

   - ## 概要 - 変更の目的と背景
   - ## 変更内容 - 具体的な変更点
   - ## 技術的詳細 - 実装の詳細
   - ## 動作確認 - テスト内容や確認事項
   - ## 影響範囲 - 既存機能への影響
   - ## 次のステップ - 後続作業（必要に応じて）

5. **PRの作成**：
   - ベースブランチの指定（引数で指定されない場合はmaster）
   - `gh pr create --title "タイトル" --body "説明" --base ベースブランチ`

### 3. **PR更新モード（既存PRがある場合）**

1. **PR番号の特定**：

   - 現在のブランチに関連するPRを自動検出
   - 複数ある場合は最新のものを選択

2. **変更内容の分析**：

   - 最新の変更内容を分析
   - 既存のPRタイトル・説明と比較

3. **タイトルと説明の更新**：

   - 最新の変更内容を反映したタイトルを生成
   - 既存の重要な情報を保持しつつ説明を更新
   - ## 更新履歴セクションを追加

4. **PRの更新**：
   - `gh pr edit PR番号 --title "新タイトル" --body "更新された説明"`

## 引数の処理

引数の解釈は文脈に応じて自動判定：

```bash
# 引数なし → 自動判定でPR作成/更新
/pr-manager

# ブランチ名 → PR作成時のベースブランチとして使用
/pr-manager develop
/pr-manager feature/base-branch

# 数字 → 既存PRの更新（PR番号として解釈）
/pr-manager 1
/pr-manager 42
```

引数の判定ルール：

- 数字のみ → PR番号として解釈（更新モード強制）
- 文字列 → ベースブランチ名として解釈（作成モード）
- 引数なし → 自動判定

## 実行例

```bash
# 状況確認
git status
gh pr list --head $(git branch --show-current)

# 条件分岐
if [既存PRあり]; then
  # PR更新
  gh pr edit PR番号 --title "新タイトル" --body "更新説明"
else
  # 必要に応じてプッシュ
  git push -u origin $(git branch --show-current)
  # PR作成
  gh pr create --title "タイトル" --body "説明" --base master
fi
```

## エラーハンドリング

- **変更がない場合**: 警告を表示してPR作成/更新をスキップ
- **リモートアクセスエラー**: 認証状況を確認するよう案内
- **権限エラー**: 適切な権限設定を案内
- **ネットワークエラー**: 接続状況を確認するよう案内

## 成功時の出力

作成/更新完了後に以下を表示：

- 実行されたアクション（作成 or 更新）
- PR番号とタイトル
- PRのURL
- 次のステップの提案（レビュー依頼など）

## 注意事項

- 実行前に必ず現在の状況を確認してください
- 重要な変更の場合は、手動でのレビューを推奨します
- 自動生成されたタイトル・説明は必要に応じて手動調整してください
- 引数として提供された値は適切に解釈されます：$ARGUMENTS

--- End Command ---
