# ドキュメント

このディレクトリには、CDK Go Blog APIを**構築する**ための手順書が含まれています。

> **注意**: アプリケーションの**使用方法**については、[ルートのREADME.md](../README.md)を参照してください。

> **💡 Docker開発環境について**: このリポジトリには統一されたDocker開発環境（Go 1.23）が同梱されています。Getting StartedのGo導入は任意で、`make setup-dev`で統一環境を利用できます。

## 📚 ドキュメント構成

### 🚀 Getting Started（初期構築）

ゼロからアプリケーションを構築するための手順です。順番に実行してください。

1. [環境準備](./getting-started/01-prerequisites.md) - Docker, Go, Node.js等の必要ツールのインストール
2. [LocalStack セットアップ](./getting-started/02-localstack-setup.md) - LocalStackの起動と基本検証
3. [CDK プロジェクト](./getting-started/03-cdk-project.md) - CDKプロジェクトの初期化
4. [Lambda 開発](./getting-started/04-lambda-development.md) - Go Lambdaの作成
5. [CDK スタック](./getting-started/05-cdk-stack.md) - S3/Lambda/API Gatewayの実装
6. [デプロイ](./getting-started/06-deployment.md) - Bootstrap & デプロイ

### 📖 Guides（運用・操作）

構築後の運用や操作に関するガイドです。

- [API 使用方法](./guides/api-usage.md) - CRUD操作のAPIエンドポイント使用例
- [運用手順](./guides/operations.md) - 更新・ログ確認・破棄の手順
- [トラブルシューティング](./guides/troubleshooting.md) - よくある問題と解決策
- [リファクタリング手順書](./guides/refactoring.md) - 本格的なプロダクション対応への改善手順

### 📋 Reference（リファレンス）

拡張や詳細実装に関するリファレンス情報です。

- [CRUD Lambda](./reference/crud-lambda.md) - 完全版Lambda実装
- [Makefile タスク](./reference/makefile-tasks.md) - ビルド・デプロイタスクの詳細
- [拡張ガイド](./reference/extensions.md) - よくある構成拡張

## 🎯 想定読者

- **初回構築者**: Getting Started を順番に実行
- **運用担当者**: Guides を参照
- **開発者**: Reference で詳細実装を確認
- **リファクタリング担当者**: [リファクタリング手順書](./guides/refactoring.md) で本格的な改善を実施

## 🔄 将来の拡張

この構造は将来的な機能拡張を想定しています：

- `guides/authentication.md` - 認証機能追加
- `guides/cicd.md` - CI/CDパイプライン
- `guides/monitoring.md` - モニタリング・ロギング
- `advanced/` - 高度なトピック

## ❓ サポート

問題が発生した場合は、まず[トラブルシューティング](./guides/troubleshooting.md)を確認してください。
