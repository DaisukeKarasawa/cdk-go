# 運用手順

## 概要

実装更新や確認、リソース破棄の方法を説明します。

**目的**: 変更の反映、問題発生時の原因追跡、不要リソースのクリーンアップができ、ローカルの環境を健全に保つ

## 更新対象別の手順

### 1. Lambda コード変更（`lambda/cmd/blog/main.go`）

**影響**: Lambda の実行バイナリが変わるため、再ビルドとZIP再生成が必要

**推奨手順（Docker環境）**:

```bash
# Docker環境でビルド→ZIP
make build-docker

# デプロイ
cdklocal deploy --require-approval never
```

**代替手順（ローカルGo環境）**:

```bash
# ビルド→ZIP
mkdir -p dist/blog
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/blog/bootstrap lambda/cmd/blog
( cd dist/blog && zip -j ../blog.zip bootstrap )

# デプロイ
cdklocal deploy --require-approval never
```

**期待結果**: 新しいロジックがAPI経由で反映（エンドポイントURL/RestApiIdは継続利用）

### 2. CDK スタック変更（`cdk-go.go` や スタックファイル）

**影響**: インフラ定義が変わるため、synth→deployが必要（破壊的変更は注意）

**手順**:

```bash
cdklocal synth
cdklocal deploy --require-approval never
```

**期待結果**: CloudFormation相当の差分適用でLocalStackのリソースが更新

### 3. LocalStack 設定変更（`docker-compose.yml`）

**影響**: LocalStackで有効化されるサービスや設定が変わる

**手順**:

```bash
docker compose down
docker compose up -d

# 必要に応じて bootstrap からやり直し
cdklocal bootstrap aws://000000000000/ap-northeast-1
cdklocal deploy --require-approval never
```

**期待結果**: 追加したサービス（例: ssm, sts, ecr）が有効化され、CDKのbootstrap/deployが正常化

### 4. Node ツール変更（`package.json`/`node_modules`）

**影響**: `cdklocal` が内部で `aws-cdk` を解決できない場合がある

**手順（推奨: ローカル導入+npx）**:

```bash
npm install -D aws-cdk aws-cdk-local
npx cdklocal --version
npx cdk --version
```

**代替（グローバル）**:

```bash
npm install -g aws-cdk aws-cdk-local
export NODE_PATH=$(npm root -g)  # 必要に応じて
cdklocal --version
cdk --version
```

**期待結果**: `cdklocal` 実行時の MODULE_NOT_FOUND が解消

## ログ・監視

### Lambda 実行ログ

LocalStackのCloudWatch Logsエミュレーションを使用：

```bash
# ログ グループ一覧
awslocal logs describe-log-groups

# Lambda ログのリアルタイム監視
awslocal logs tail "/aws/lambda/BlogApi" --follow
```

### API Gateway 呼び出し確認

`curl` と `jq` を活用した動作確認：

```bash
# REST API ID 取得
REGION=${AWS_DEFAULT_REGION:-us-east-1}
REST_API_ID=$(awslocal --region "$REGION" apigateway get-rest-apis | jq -r '.items[0].id')
BASE="http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_"

# 基本動作確認
curl -s "${BASE}/posts" | jq .
curl -s "${BASE}/posts/1" | jq .
```

### LocalStack サービス状況

```bash
# LocalStack の健全性確認
curl http://localhost:4566/_localstack/health | jq

# Docker コンテナ状況
docker compose ps
```

## 記事データの検証（CRUD版）

完全なCRUD操作の動作確認：

```bash
# REST API ID 取得
REST_API_ID=$(awslocal apigateway get-rest-apis | jq -r '.items[0].id')
BASE="http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_"

# 作成→一覧→取得→更新→削除→削除確認
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"id":1,"title":"Hello","content":"# Hello from API\nThis is markdown content."}' \
  "${BASE}/posts" | jq .

curl -s "${BASE}/posts" | jq .
curl -s "${BASE}/posts/1" | jq .

curl -s -X PUT -H "Content-Type: application/json" \
  -d '{"title":"Hello (updated)","content":"# Updated\nNew content."}' \
  "${BASE}/posts/1" | jq .

curl -i -s -X DELETE "${BASE}/posts/1" | head -n1
curl -s -o /dev/null -w "%{http_code}\n" "${BASE}/posts/1"
```

**期待結果**: 各操作が期待ステータス/レスポンスで完了し、S3の `posts/` 配下が連動

## 破棄・クリーンアップ

### スタック破棄

```bash
cdklocal destroy --force
```

**効果**:

- CDKで作成したすべてのリソース（S3、Lambda、API Gateway、IAM）を削除
- LocalStack内のデータもクリア

### LocalStack 全体の停止・再起動

```bash
# 停止
docker compose down

# 再起動
docker compose up -d
```

**効果**:

- LocalStack内のすべてのデータ・設定がリセット
- 再起動後は bootstrap から実行が必要

### 部分的なクリーンアップ

```bash
# S3 バケット内のデータのみ削除
POSTS_BUCKET=$(awslocal s3 ls | awk '{print $3}' | grep -i blogposts | head -n1)
awslocal s3 rm s3://$POSTS_BUCKET/posts/ --recursive

# Lambda ログのクリア
awslocal logs delete-log-group --log-group-name "/aws/lambda/BlogApi"
```

## Makefile を使った運用（推奨）

プロジェクトルートに `Makefile` がある場合：

**Docker環境での開発**:

```bash
# 開発環境セットアップ
make setup-dev

# Lambda ビルド（Docker環境）
make build-docker

# テスト実行（Docker環境）
make test-docker

# デプロイ
make deploy

# ログ監視
make logs

# 破棄
make destroy

# 開発環境クリーンアップ
make clean-dev
```

**従来のローカル環境**:

```bash
# Lambda ビルド
make build-lambda

# デプロイ
make deploy

# ログ監視
make logs

# 破棄
make destroy
```

詳細は[Makefileタスク](../reference/makefile-tasks.md)を参照してください。

## Go モジュール管理（go mod tidy / go get）

Go の依存管理は `go.mod` に依存します。同じ目的でも Docker/非Docker の2通りを用意しています。

- Docker環境（推奨）:

```bash
# 依存整理
docker compose exec go-dev go mod tidy

# 依存追加（例）
docker compose exec go-dev go get github.com/aws/aws-lambda-go@latest
```

- ローカルGo（非Docker）:

```bash
# Goバージョンを確認（Go 1.23 以上を推奨）
go version

# 依存整理
go mod tidy

# 依存追加（例）
go get github.com/aws/aws-lambda-go@latest
```

- 互換性のメモ:
  - `go.mod` の `go` はメジャー.マイナーのみ（`1.23`）。`1.23.0` は無効です。
  - `toolchain` ディレクティブは Go 1.21+ で利用可能。古いGoで `unknown directive: toolchain` が出る場合は、
    1) Docker の `go-dev` 環境で実行する、または 2) 一時的に `toolchain` 行を外して実行してください。

- `go.mod` 記述の例（2パターン）:

  - 最新ツールチェインを明示:

    ```
    go 1.23
    toolchain go1.23.12
    ```

  - 互換性優先（古いGoでも動かす）:

    ```
    go 1.23
    # toolchain 行は一時的に外しても可
    ```

## トラブル時の対処

### デプロイが失敗する

1. **LocalStack の状況確認**:

   ```bash
   docker compose ps
   curl http://localhost:4566/_localstack/health | jq
   ```

2. **CDK の状況確認**:

   ```bash
   cdklocal synth  # テンプレート生成確認
   ```

3. **アセットの確認**:
   ```bash
   ls -la dist/blog.zip  # Lambda ZIP の存在確認
   ```

### API が応答しない

1. **Lambda ログ確認**:

   ```bash
   awslocal logs tail "/aws/lambda/BlogApi" --follow
   ```

2. **API Gateway 設定確認**:

   ```bash
   awslocal apigateway get-rest-apis | jq
   ```

3. **権限確認**:
   ```bash
   # S3 バケットの存在確認
   awslocal s3 ls
   ```

詳細なトラブルシューティングは[こちら](./troubleshooting.md)を参照してください。
