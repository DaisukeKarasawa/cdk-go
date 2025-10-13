# CDK プロジェクト初期化

## 概要

Go 言語で CDK アプリの雛形を作成します。

**目的**: CDKの標準構成（`bin/`、`cdk.json`、スタック雛形）が生成され、このディレクトリが以後の開発・デプロイの基点になる

**リスク**: Node/npm 不足、Go 環境の PATH 未設定

## 手順

### 1. CDK CLI の確認

```bash
# CDK CLI を（必要なら）グローバル導入
# 既に `cdk --version` が出る場合はこの手順をスキップ
npm install -g aws-cdk || true
cdk --version  # v2.x
```

### EEXIST エラーの対処

既存ファイルありエラーが出る場合：

```bash
# 例: npm error EEXIST: file already exists, /opt/homebrew/bin/cdk

# 対処1: 上書き（注意）
npm install -g aws-cdk --force

# 対処2: 既存のcdkを一旦アンインストール
npm uninstall -g aws-cdk && npm install -g aws-cdk
```

### 2. Go CDK アプリの作成

プロジェクト直下で実行（空ディレクトリで実行してください）：

```bash
cd <PROJECT_ROOT>

# 既存ディレクトリが非空の場合は、別名で新規作成してから移動
# 例: mkdir my-cdk-app && cd my-cdk-app

cdk init app --language go
```

### 3. 依存解決

```bash
go mod tidy
```

## 生成される構成

CDK初期化により以下のファイル・ディレクトリが生成されます：

```text
.
├── bin/                    # エントリポイント（App 定義）
├── cdk.json               # CDK 実行設定
├── <プロジェクト名>_stack.go  # スタック定義置き場（例: cdk_go_stack.go）
├── go.mod                 # Go モジュール定義
├── go.sum                 # Go 依存関係ロック
└── README.md              # CDK生成のREADME
```

### 主要ファイルの役割

- **`bin/`**: CDKアプリのエントリポイント
- **`cdk.json`**: CDK実行時の設定（アプリのエントリポイント指定等）
- **`*_stack.go`**: スタック定義（S3、Lambda、API Gateway等のリソース定義）

## トラブルシューティング

### 非空ディレクトリエラー

```bash
# エラー: cannot be run in a non-empty directory
# 解決策: 新しいディレクトリを作成
mkdir my-cdk-app && cd my-cdk-app && cdk init app --language go
```

### Go モジュール問題

```bash
# go mod tidy でエラーが出る場合
go clean -modcache
go mod tidy
```

## 確認

初期化が成功すると、以下のコマンドでスタックを確認できます：

```bash
# スタック一覧
cdk list

# CloudFormationテンプレート生成（合成）
cdk synth
```

## 次のステップ

CDKプロジェクトの初期化が完了したら、[Lambda 開発](./04-lambda-development.md)に進んでください。
