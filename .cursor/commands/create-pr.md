---
allowed-tools: Bash(git status:*), Bash(git diff:*), Bash(git log:*), Bash(git push:*), Bash(gh pr create:*), Bash(gh pr list:*), Bash(gh pr view:*)
description: 現在のブランチからPRを作成し、コミット内容を分析して適切なタイトルと説明を自動生成する
argument-hint: [optional-base-branch]
---

# PR作成コマンド

現在のブランチからPull Requestを作成し、コミット内容を分析して適切なタイトルと説明を自動生成します。

## 現在の状況

### Git ステータス

!`git status --porcelain`

### 現在のブランチ

!`git branch --show-current`

### コミット履歴（現在のブランチ vs master）

!`git log --oneline master..HEAD --format="%h %s"`

### 変更差分の統計

!`git diff master..HEAD --stat`

### 変更されたファイルの詳細

!`git diff master..HEAD --name-status`

## あなたのタスク

上記の情報を基に、以下の手順でPRを作成してください：

1. **変更内容の分析**：

   - 変更されたファイルの種類と内容を確認
   - 変更の性質（新機能、バグ修正、リファクタリングなど）を判断
   - 影響範囲と重要度を評価

2. **適切なタイトルの生成**：

   - Conventional Commitsの形式に従う
   - `feat:` - 新機能の追加
   - `fix:` - バグ修正
   - `docs:` - ドキュメントの変更
   - `style:` - コードスタイルやフォーマットの修正
   - `refactor:` - コードのリファクタリング
   - `test:` - テストの追加や修正
   - `chore:` - ビルドプロセスや補助ツールの変更
   - `perf:` - パフォーマンス改善
   - `ci:` - CI/CD関連の変更

3. **詳細な説明の作成**：

   - ## 概要 - 変更の目的と背景
   - ## 変更内容 - 具体的な変更点
   - ## 技術的詳細 - 実装の詳細（必要に応じて）
   - ## 動作確認 - テスト内容や確認事項
   - ## 影響範囲 - 既存機能への影響
   - ## 次のステップ - 後続作業（必要に応じて）

4. **PRの作成**：
   - ベースブランチの指定（引数で指定されない場合はmaster）
   - 生成したタイトルと説明でPRを作成
   - 作成されたPRのURLを表示

## 実行例

```bash
# 現在のブランチの変更内容を確認
git status
git log --oneline master..HEAD

# PRを作成（masterブランチに対して）
gh pr create --title "適切なタイトル" --body "詳細な説明" --base master

# 作成されたPRを確認
gh pr view --web
```

## 注意事項

- PRを作成する前に、必ず変更内容を確認してください
- タイトルは50文字以内で簡潔にまとめてください
- 説明は将来の開発者（自分を含む）が理解しやすいように書いてください
- 引数として特定のベースブランチが提供された場合は、それを使用してください：$ARGUMENTS
- 既存のPRがある場合は、新しいPRを作成せずに警告を表示してください

## ベースブランチの指定

引数でベースブランチを指定できます：

```bash
# masterブランチに対してPRを作成（デフォルト）
/create-pr

# developブランチに対してPRを作成
/create-pr develop

# 特定のブランチに対してPRを作成
/create-pr feature/base-branch
```

## エラーハンドリング

- リモートブランチが存在しない場合は、pushしてからPRを作成
- 既存のPRがある場合は、更新するかどうかを確認
- 変更がない場合は、PRを作成せずに警告を表示

--- End Command ---
