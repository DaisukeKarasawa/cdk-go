# Makefile タスク

## 概要

Lambdaのビルド（Linux/amd64用bootstrap生成→ZIP化）と、cdklocalの `bootstrap/synth/deploy/destroy/logs` を定型タスク化します。

**目的**: ワンコマンドでビルド〜デプロイが行え、ヒューマンエラー（ZIP未作成・環境変数未設定など）の抑止と開発ループの短縮が可能

## Makefile 実装

プロジェクトルートに `Makefile` を作成：

```makefile
SHELL := /bin/bash

# Go Lambda をビルドしてZIP化（Linux/amd64でビルド、bootstrap実行形式）
build-lambda:
	mkdir -p dist/blog
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/blog/bootstrap lambda/cmd/blog
	cd dist/blog && zip -j ../blog.zip bootstrap

bootstrap:
	cdklocal bootstrap aws://000000000000/ap-northeast-1

deploy: build-lambda
	cdklocal deploy --require-approval never

destroy:
	cdklocal destroy --force

synth:
	cdklocal synth

logs:
	awslocal logs tail "/aws/lambda/BlogApi" --follow
```

## タスク詳細

### build-lambda

**目的**: Lambda関数をLinux/amd64用にクロスコンパイルしてZIP化

**処理内容**:

1. `dist/blog/` ディレクトリを作成
2. `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` でクロスコンパイル
3. 実行ファイル名を `bootstrap` に設定（Lambda PROVIDED_AL2 ランタイム要件）
4. `dist/blog.zip` にZIP化

**使用例**:

```bash
make build-lambda
```

**出力**:

- `dist/blog/bootstrap` - Linux用実行ファイル
- `dist/blog.zip` - デプロイ用ZIPアーカイブ

### bootstrap

**目的**: CDK bootstrap スタックをLocalStackに作成

**処理内容**:

- LocalStack固定のアカウントID（000000000000）でbootstrap実行
- CDKデプロイに必要なS3バケット、IAMロール等を作成

**使用例**:

```bash
make bootstrap
```

**注意**: 初回のみ実行が必要（LocalStackを完全リセットした場合は再実行）

### deploy

**目的**: Lambda ビルド + CDK デプロイを一括実行

**処理内容**:

1. `build-lambda` タスクを依存として実行
2. `cdklocal deploy` でスタックをデプロイ

**使用例**:

```bash
make deploy
```

**利点**: ビルド忘れを防止し、常に最新のコードをデプロイ

### destroy

**目的**: CDKスタックを完全削除

**処理内容**:

- 作成されたすべてのリソース（S3、Lambda、API Gateway、IAM）を削除
- `--force` オプションで確認なしで実行

**使用例**:

```bash
make destroy
```

**注意**: データも含めて完全削除されます

### synth

**目的**: CloudFormationテンプレートの生成（合成）

**処理内容**:

- CDKアプリからCloudFormationテンプレートを生成
- `cdk.out/` ディレクトリに出力

**使用例**:

```bash
make synth
```

**用途**: デプロイ前の設定確認、テンプレートの検証

### logs

**目的**: Lambda関数のログをリアルタイム監視

**処理内容**:

- CloudWatch Logs（LocalStack）から Lambda ログを取得
- `--follow` オプションでリアルタイム表示

**使用例**:

```bash
make logs
```

**終了**: `Ctrl+C` で監視を停止

## 開発ワークフロー

### 初回セットアップ

```bash
# 1. LocalStack 起動
docker compose up -d

# 2. Bootstrap（初回のみ）
make bootstrap

# 3. 初回デプロイ
make deploy
```

### 日常的な開発ループ

```bash
# 1. コード変更
# lambda/cmd/blog/main.go を編集

# 2. デプロイ（ビルドも自動実行）
make deploy

# 3. ログ確認（別ターミナル）
make logs

# 4. API テスト
curl -s "http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_/posts"
```

### デバッグ時

```bash
# テンプレート確認
make synth

# ログ監視
make logs

# 完全リセット
make destroy
docker compose down && docker compose up -d
make bootstrap
make deploy
```

## カスタマイズ

### 環境変数の設定

```makefile
# 環境変数を明示的に設定
deploy: build-lambda
	AWS_DEFAULT_REGION=ap-northeast-1 cdklocal deploy --require-approval never
```

### 複数環境対応

```makefile
# 開発環境
deploy-dev: build-lambda
	cdklocal deploy --require-approval never --context env=dev

# ステージング環境
deploy-staging: build-lambda
	cdklocal deploy --require-approval never --context env=staging
```

### テストタスクの追加

```makefile
test:
	go test ./...

test-api: deploy
	./scripts/api-test.sh

lint:
	golangci-lint run
```

### ヘルプの追加

```makefile
help:
	@echo "Available targets:"
	@echo "  build-lambda  - Build Lambda function for Linux/amd64"
	@echo "  bootstrap     - Bootstrap CDK stack"
	@echo "  deploy        - Build and deploy stack"
	@echo "  destroy       - Destroy stack"
	@echo "  synth         - Generate CloudFormation template"
	@echo "  logs          - Follow Lambda logs"
```

## トラブルシューティング

### make: command not found

```bash
# macOS
brew install make

# または GNU make を使用
gmake deploy
```

### Permission denied

```bash
# Makefile の実行権限確認
ls -la Makefile

# 必要に応じて権限付与
chmod +x Makefile
```

### ZIP作成エラー

```bash
# zip コマンドの確認
which zip

# macOS で zip がない場合
brew install zip
```

### クロスコンパイルエラー

```bash
# Go のクロスコンパイル対応確認
go env GOOS GOARCH

# 必要に応じて Go を再インストール
brew reinstall go
```

## 利点

1. **一貫性**: 常に同じ手順でビルド・デプロイ
2. **効率性**: ワンコマンドで複数の処理を実行
3. **エラー防止**: 依存関係を明示的に定義
4. **ドキュメント化**: タスクの内容が Makefile に記録
5. **チーム開発**: 統一された開発フロー

## 代替手段

### npm scripts

```json
{
  "scripts": {
    "build": "make build-lambda",
    "deploy": "make deploy",
    "logs": "make logs"
  }
}
```

### シェルスクリプト

```bash
#!/bin/bash
# scripts/deploy.sh
set -e

echo "Building Lambda..."
mkdir -p dist/blog
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/blog/bootstrap lambda/cmd/blog
cd dist/blog && zip -j ../blog.zip bootstrap

echo "Deploying..."
cdklocal deploy --require-approval never
```

Makefileは多くの開発者に馴染みがあり、依存関係の管理が得意なため推奨します。
