# デプロイ

## 概要

定義したスタックを LocalStack に反映します。

**目的**: LocalStack上にS3/Lambda/API Gatewayが構築され、以後のAPI呼び出し・記事投入が可能になる

## 手順

### 1. 事前準備

Lambda ZIPアセットを作成（未作成の場合）：

```bash
mkdir -p dist/blog
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/blog/bootstrap lambda/cmd/blog
( cd dist/blog && zip -j ../blog.zip bootstrap )
```

### 2. Bootstrap

CDK のデプロイに必要なブートストラップスタックを LocalStack に作成：

```bash
# アカウントIDは LocalStack 固定の 000000000000 を使用
echo $AWS_DEFAULT_REGION  # ap-northeast-1 が前提

cdklocal bootstrap aws://000000000000/ap-northeast-1
```

**Bootstrap の役割**:

- CDKがデプロイ時に利用するアセット用バケットやロール等の基盤スタック作成
- LocalStackの仮想アカウント（000000000000）に作成
- 以降の `cdklocal deploy` でアセット（Lambdaコード等）を転送・参照可能

### 3. 合成（テンプレート生成）

```bash
cdklocal synth
```

CloudFormationテンプレートが `cdk.out/` ディレクトリに生成されます。

### 4. デプロイ

```bash
cdklocal deploy --require-approval never
```

**実行内容**:

- CloudFormationスタックの作成/更新
- S3バケットの作成
- Lambda関数のデプロイ
- API Gatewayの作成
- IAM権限の設定

### 5. デプロイ結果の確認

デプロイ完了後、出力ログに重要な情報が表示されます：

```text
✅  CdkGoStack

Stack ARN:
arn:aws:cloudformation:ap-northeast-1:000000000000:stack/CdkGoStack/...

Outputs:
CdkGoStack.BlogApiGatewayEndpoint = https://xxxxxxxxxx.execute-api.ap-northeast-1.amazonaws.com/prod/
```

## API エンドポイントの取得

### 方法1: AWS CLI で取得

```bash
# リージョンの決定（Stack ARNのリージョンに合わせる / 既定us-east-1）
REGION=${AWS_DEFAULT_REGION:-us-east-1}

# REST API ID の取得
REST_API_ID=$(awslocal --region "$REGION" apigateway get-rest-apis | jq -r '.items[0].id')
echo "REST API ID: $REST_API_ID"

# LocalStack のエンドポイント形式
BASE="http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_"
echo "API Base URL: $BASE"
```

### 方法2: CDK Output で出力

CDKスタックに以下を追加して再デプロイ：

```go
awscdk.NewCfnOutput(stack, awsString("ApiEndpoint"), &awscdk.CfnOutputProps{
    Value: awsString(fmt.Sprintf("http://localhost:4566/restapis/%s/prod/_user_request_/", *api.RestApiId())),
})
```

## API URL 形式（LocalStack）

```text
# 一覧取得
http://localhost:4566/restapis/{restApiId}/prod/_user_request_/posts

# 個別取得
http://localhost:4566/restapis/{restApiId}/prod/_user_request_/posts/{id}
```

## 動作確認

### 基本的なAPI呼び出し

```bash
# REST API ID を取得
REGION=${AWS_DEFAULT_REGION:-us-east-1}
REST_API_ID=$(awslocal --region "$REGION" apigateway get-rest-apis | jq -r '.items[0].id')
BASE="http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_"

# 1) 一覧（GET /posts）
curl -s "${BASE}/posts" | jq .

# 2) 1件取得（GET /posts/1）
curl -s "${BASE}/posts/1"
```

### 期待結果

- **GET /posts**: JSON配列（サンプル1件）
- **GET /posts/{id}**: Markdownテキスト

## 重要な注意事項

### リージョン整合性

- デプロイとAPI取得は同一リージョンで行ってください
- 基本は `REGION=${AWS_DEFAULT_REGION:-us-east-1}` として、取得系コマンドでは `--region "$REGION"` を付けると安全
- 出力された Stack ARN に含まれるリージョンが実際に使われたリージョン

### アセットの更新

Lambda コードを変更した場合は、再ビルド→再デプロイが必要：

```bash
# Lambda 再ビルド
mkdir -p dist/blog
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/blog/bootstrap lambda/cmd/blog
( cd dist/blog && zip -j ../blog.zip bootstrap )

# 再デプロイ
cdklocal deploy --require-approval never
```

## 次のステップ

デプロイが完了したら、[API 使用方法](../guides/api-usage.md)でCRUD操作を確認してください。

実運用レベルのCRUD機能が必要な場合は、[CRUD対応Lambda](../reference/crud-lambda.md)を参照して実装を置き換えてください。
