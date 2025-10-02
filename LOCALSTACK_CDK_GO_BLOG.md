# ローカルで動くサーバーレスブログ（CDK + Go + LocalStack）手順書

## 方針

- CDK（Go）で API Gateway + Lambda（Go）+ S3 の最小構成によるブログAPIを作成し、LocalStack 上にデプロイして AWS を使わずにローカルで動作検証します。
- まず作業項目をリストアップし、その後に各項目の詳細手順を順番に記載します。
- すべてのコマンドは再現性を重視した形で提示し、macOS（darwin 24.6.0）前提で説明します。

## 参考資料（必ず参照）

- LocalStack Docs（設定・エンドポイント・ツール連携の基礎）
  - <https://docs.localstack.cloud/>
- AWS CDK v2 Developer Guide（CDKの基本・ブートストラップ・デプロイ）
  - <https://docs.aws.amazon.com/cdk/v2/guide/home.html>
- AWS CDK Construct Library（各サービスのConstruct仕様）
  - <https://docs.aws.amazon.com/cdk/api/v2/docs/aws-construct-library.html>
- Zenn: AWS CDKをLocalStackで練習（API Gateway + Lambda + S3 のローカル検証の実例と cdklocal の使い方の参考）
  - <https://zenn.dev/okojomoeko/articles/4584312c51810d>
- Zenn: CDK × LocalStack 関連の知見（cdklocal/awslocal やエンドポイントの扱いの参考）
  - <https://zenn.dev/kin/articles/d22f9b30263afb>

---

## タスク分類（本手順の適応プロセス）

- 分類: 🟡 標準タスク（機能追加/複数ファイルの新規作成を想定）

### 実行計画（チェックリスト）

1. [独立] 前提ツールのインストール 🟢
2. [独立] LocalStack の起動と基本検証 🟢
3. [依存:1] CDK（Go）プロジェクトの初期化
4. [依存:3] Go Lambda の雛形作成
5. [依存:3-4] CDK スタック定義（S3 / Lambda / API Gateway）
6. [依存:2-5] LocalStack への bootstrap
7. [依存:6] LocalStack へデプロイ
8. [依存:7] 記事 CRUD（API 推奨）/ S3 直接（任意）
9. [依存:7-8] CRUD 動作確認（curl / awslocal）
10. [独立] 運用コマンド（更新/ログ/破棄）
11. [独立] トラブルシューティング

---

## 作業項目一覧（先に全体像）

- 環境準備
  - Docker, Docker Compose, Go, Node.js, AWS CLI, LocalStack, awslocal, cdklocal, jq
- LocalStack の起動と基本検証
- CDK（Go）プロジェクト初期化
- Go Lambda の作成（API 用ハンドラ）
- CDK スタックの実装（S3, Lambda, API Gateway の連携）
- CDK bootstrap（LocalStack 向け）
- デプロイおよび API エンドポイントの取得
- 記事データの CRUD（API 推奨）/ S3 直接（任意）
- 動作確認（API 呼び出し / S3 確認）
- 運用方法（更新/ログ/破棄）
- トラブルシューティング

---

## 詳細手順

### 1. 環境準備（インストール）

- 目的: LocalStack + CDK（Go）でローカル完結の IaC/サーバーレス実行環境を整える
- リスク: バージョン不整合や PATH の競合
- 実際に行うこと: Docker/ComposeでLocalStackを動かす基盤、GoとNodeでCDK・Lambdaのビルド/CLI環境、awscli/awslocalでAWS APIのローカル操作、cdklocalでCDKのLocalStack向けデプロイ実行環境を整備します。
- 結果: クラウドに接続せずに、ローカルだけでAWS互換のAPIを呼び出し、CDKアプリの合成・デプロイ・検証が可能になります。

手順:

```bash
# 1) Homebrew 更新
brew update

# 2) Docker Desktop（未インストールなら）
#   https://www.docker.com/products/docker-desktop/ からインストール
#   インストール後に Docker Desktop を起動しておく

# 3) Go（1.21+ 推奨）
brew install go

go version  # 例: go version go1.22.x darwin/arm64 or amd64

# 4) Node.js（CDK CLI 用 / LTS推奨）
brew install node

node -v  # 例: v20.x
npm -v   # 例: 10.x

# 5) AWS CLI v2（任意。awslocal だけでもよい）
brew install awscli
aws --version

# 6) Python ツール（pipx経由で LocalStack ラッパー導入推奨）
brew install pipx
pipx ensurepath

# 7) awslocal / cdklocal の導入
pipx install awscli-local    # awslocal

# 推奨: プロジェクトにローカル導入（再現性が高い）
npm init -y >/dev/null 2>&1 || true
npm install -D aws-cdk aws-cdk-local
# その場実行: npx cdklocal <cmd> / npx cdk <cmd>

# 代替（グローバル導入）:
# npm install -g aws-cdk aws-cdk-local
# export NODE_PATH=$(npm root -g)  # cdklocal が aws-cdk を解決できない場合に必要
# Homebrew の aws-cdk を使う場合（brew 経由で CLI を導入したとき）:
# export NODE_PATH="$(brew --prefix aws-cdk)/libexec/lib/node_modules:$NODE_PATH"

# 注意: `npx install -g aws-cdk-local` は無効。グローバル化は `npm install -g` を使用。

# 動作確認
npx cdklocal --version
npx cdk --version  # ローカル導入時
cdklocal --version # グローバル導入時

# 8) jq（レスポンス整形用。任意）
brew install jq

# 9) 環境変数（ローカル用ダミー資格情報）
#    LocalStack は任意の資格情報で可。固定しておくと便利。
export AWS_ACCESS_KEY_ID=dummy
export AWS_SECRET_ACCESS_KEY=dummy
export AWS_DEFAULT_REGION=ap-northeast-1

# プロファイルを作る場合（任意）
aws configure --profile localstack <<EOF
dummy
dummy
ap-northeast-1
json
EOF
```

補足:

- LocalStack 用のラッパー `awslocal`/`cdklocal` を使うと `--endpoint-url` の指定が不要になり、設定漏れが減ります（参考: <https://zenn.dev/okojomoeko/articles/4584312c51810d>, <https://zenn.dev/kin/articles/d22f9b30263afb>）。

---

### 2. LocalStack の起動と基本検証

- 目的: LocalStack を Docker 上で起動し、S3 等の基本動作を確認
- リスク: ポート競合（デフォルト: 4566）
- 実際に行うこと: docker composeでLocalStackコンテナを起動し、 `awslocal s3` でS3のバケット作成と一覧取得を行います。
- 結果: LocalStackが正しく起動しAWS互換エンドポイントが機能していること、ダミー資格情報での操作が通ることを確認できます。

手順（docker compose 推奨）:

```yaml
# docker-compose.yml（プロジェクト直下に作成）
# LocalStack Community 版の最小構成
# CloudFront 等 Pro 専用は利用しません
services:
  localstack:
    image: localstack/localstack:latest
    container_name: localstack
    ports:
      - "4566:4566"   # Edge port
      - "4571:4571"
    environment:
      - SERVICES=s3,lambda,apigateway,cloudformation,iam,logs,ssm,sts,ecr
      - DEBUG=1
      - AWS_DEFAULT_REGION=ap-northeast-1
      # 任意: Lambda 実行エンジン（docker/reuse-enabled など）
      # - LAMBDA_EXECUTOR=docker
    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock"
```

```bash
# 起動
docker compose up -d

# 稼働確認
docker compose ps

# S3 の疎通確認（バケット作成→一覧）
awslocal s3 mb s3://blog-posts
awslocal s3 ls
```

補足:

- CDK の bootstrap では `SSM`（Parameter Store）, `ECR`, `STS` が使われるため、`SERVICES` に `ssm, sts, ecr` を含めてください。
- LocalStack 上のリソースはすべてローカルに閉じます（課金なし）。

---

### 3. CDK（Go）プロジェクト初期化

- 目的: Go 言語で CDK アプリの雛形を作成
- リスク: Node/npm 不足、Go 環境の PATH 未設定
- 実際に行うこと: CDK CLIの導入確認後、空ディレクトリにCDKアプリを作成し、 `go mod tidy` でGo依存を解決します。
- 結果: CDKの標準構成（ `bin/` 、 `cdk.json` 、スタック雛形）が生成され、このディレクトリが以後の開発・デプロイの基点になります。

手順:

```bash
# CDK CLI を（必要なら）グローバル導入
# 既に `cdk --version` が出る場合はこの手順をスキップ
# ※ cdklocal でも init は可能ですが、ここでは cdk CLI を使います
npm install -g aws-cdk || true
cdk --version  # v2.x

# EEXIST（既存ファイルあり）エラーが出る場合の回避
# 例: npm error EEXIST: file already exists, /opt/homebrew/bin/cdk
# 対処1: 上書き（注意）
# npm install -g aws-cdk --force
# 対処2: 既存のcdkを一旦アンインストール
# npm uninstall -g aws-cdk && npm install -g aws-cdk

# Go CDK アプリの作成（プロジェクト直下で実行）
# 例: リポジトリのプロジェクトルートに移動（空ディレクトリで実行してください）
cd <PROJECT_ROOT>

# 既存ディレクトリが非空の場合は、別名で新規作成してから移動
# 例: mkdir my-cdk-app && cd my-cdk-app

cdk init app --language go

# 依存解決
go mod tidy
```

生成物の主な構成（参考）:

- `bin/` … エントリポイント（App 定義）
- `cdk.json` … CDK 実行設定
- `<プロジェクト名>_stack.go`（例: `cdk_go_stack.go`）… スタック定義置き場

---

### 4. Go Lambda の雛形作成

- 目的: ブログ API のハンドラ（Go）を作成
- リスク: Lambda 用ビルド設定の不足
- 実際に行うこと: `aws-lambda-go` を導入し、HTTPメソッドとパスで分岐する最小Lambdaハンドラ（後にCRUD版へ差し替え）を実装します。ビルドはCDKの `GoFunction` が行います。
- 結果: API Gatewayからのイベントを受け取り、ルーティングできる関数の最低限の土台ができます。

最小のブログ API（一覧と1件取得のモック）例:

```bash
# Go Lambda ランタイム依存（最小）
go get github.com/aws/aws-lambda-go@latest
```

```go
// lambda/cmd/blog/main.go
package main

import (
 "context"
 "encoding/json"
 "fmt"
 "net/http"
 "os"
 "strings"

 "github.com/aws/aws-lambda-go/events"
 "github.com/aws/aws-lambda-go/lambda"
)

type Post struct {
 ID      string `json:"id"`
 Title   string `json:"title"`
 Content string `json:"content"`
}

func handleRequest(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
 path := req.Path
 method := req.HTTPMethod

 // 簡易ルーティング
 if method == http.MethodGet && path == "/posts" {
  // 本来は S3 から一覧を構築
  posts := []Post{{ID: "hello", Title: "Hello", Content: "Hello from LocalStack"}}
  b, _ := json.Marshal(posts)
  return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(b), Headers: map[string]string{"Content-Type": "application/json"}}, nil
 }
 if method == http.MethodGet && strings.HasPrefix(path, "/posts/") {
  id := strings.TrimPrefix(path, "/posts/")
  // 本来は S3 の `posts/{id}.md` を取得して返す
  content := fmt.Sprintf("# %s\n\nThis is a mock article.", id)
  return events.APIGatewayProxyResponse{StatusCode: 200, Body: content, Headers: map[string]string{"Content-Type": "text/markdown; charset=utf-8"}}, nil
 }

 return events.APIGatewayProxyResponse{StatusCode: 404, Body: "not found"}, nil
}

func main() {
 _ = os.Setenv("AWS_SDK_LOAD_CONFIG", "1")
 lambda.Start(handleRequest)
}
```

メモ:

- 最小動作確認用の雛形です。実運用では付録AのCRUD対応版に置き換えてください。

---

### 5. CDK スタック実装（S3 / Lambda / API Gateway）

- 目的: S3（記事格納用）、Lambda（API）、API Gateway（公開）を CDK（Go）で定義
- リスク: Goバイナリのクロスコンパイル設定やアセット配置ミス
- 実際に行うこと: S3バケットを作成し、LambdaにS3の読み書き権限を付与し、事前にビルドしたZIPアセット（ `dist/lambda/blog.zip` ）を `Code.FromAsset` で参照、API GatewayでLambdaを統合してRESTエンドポイントを作ります。
- 結果: 記事データの保存先（S3）と、それにアクセスする実行関数（Lambda）、外部公開のHTTP入口（API Gateway）が1つのスタックとして連携します。

依存の追加（`go.mod` に追記される想定）:

```bash
# Option A: latest（通信環境により失敗する場合あり）
go get github.com/aws/aws-cdk-go/awscdk/v2@latest

# Option B: バージョン固定（推奨: ネットワーク起因の揺らぎ回避）
# 例）v2.219.0 に固定（必要に応じて調整してください）
# go get github.com/aws/aws-cdk-go/awscdk/v2@v2.219.0

go get github.com/aws/constructs-go/constructs/v10@latest
# awslambdagoalpha は使用せず、ビルド済みZIPアセットを配布します（下記ビルド手順参照）。

# 取得に失敗する場合は「トラブルシューティング: Goモジュール取得失敗」を参照
```

スタック例（最小構成）:

```go
// cdk_go_stack.go（プロジェクト生成時のスタックファイル名に合わせて配置）
package main

import (
 awscdk "github.com/aws/aws-cdk-go/awscdk/v2"
 "github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
 "github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
 "github.com/aws/aws-cdk-go/awscdk/v2/awss3"
 "github.com/aws/constructs-go/constructs/v10"
)

type CdkGoStackProps struct {
 awscdk.StackProps
}

func NewCdkGoStack(scope constructs.Construct, id string, props *CdkGoStackProps) awscdk.Stack {
 var sprops awscdk.StackProps
 if props != nil {
  sprops = props.StackProps
 }
 stack := awscdk.NewStack(scope, &id, &sprops)

 // S3: 記事格納用
 bucket := awss3.NewBucket(stack, awsString("BlogPosts"), &awss3.BucketProps{})

 // Lambda: 事前ビルドしたZIPアセットを使用
 fn := awslambda.NewFunction(stack, awsString("BlogApi"), &awslambda.FunctionProps{
  Runtime: awslambda.Runtime_PROVIDED_AL2(),
  Handler: awsString("bootstrap"),
  Code: awslambda.Code_FromAsset(awsString("dist/lambda/blog.zip"), nil),
  Environment: &map[string]*string{
   "POSTS_BUCKET": bucket.BucketName(),
  },
 })
 bucket.GrantReadWrite(fn, nil)

 // API Gateway: /posts, /posts/{id}
 api := awsapigateway.NewLambdaRestApi(stack, awsString("BlogApiGateway"), &awsapigateway.LambdaRestApiProps{
  Handler: fn,
 })
 _ = api // エンドポイントはデプロイ後に CfnOutput でも出力可能

 return stack
}

func awsString(v string) *string { return &v }
```

API ルーティングは Lambda 側の `APIGatewayProxyRequest.Path` で分岐しています（参考: <https://zenn.dev/okojomoeko/articles/4584312c51810d> の API Gateway + Lambda パターン）。

---

### 6. LocalStack へ bootstrap

- 目的: CDK のデプロイに必要なブートストラップスタックを LocalStack に作成
- リスク: エンドポイント設定漏れ（`cdklocal` を使えば軽減）
- 実際に行うこと: CDKがデプロイ時に利用するアセット用バケットやロール等の基盤スタック（bootstrapスタック）を、LocalStackの仮想アカウント（000000000000）に作成します。
- 結果: 以降の `cdklocal deploy` でアセット（Lambdaコード等）を転送・参照できる状態が整います。

手順:

```bash
# 事前: Code.FromAsset を使用している場合は Lambda ZIP を用意（未作成だと "Cannot find asset" で失敗）
mkdir -p dist/lambda/blog
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/lambda/blog/bootstrap lambda/cmd/blog
( cd dist/lambda/blog && zip -j ../blog.zip bootstrap )

# アカウントIDは LocalStack 固定の 000000000000 を使用
echo $AWS_DEFAULT_REGION  # ap-northeast-1 が前提

cdklocal bootstrap aws://000000000000/ap-northeast-1
```

---

### 7. デプロイ

- 目的: 定義したスタックを LocalStack に反映
- 実際に行うこと: `cdklocal synth` でCloudFormationテンプレートを生成し、 `cdklocal deploy` でLocalStackへスタックを作成/更新します。APIのRestApiIdなどの実リソースIDが確定します。
- 結果: LocalStack上にS3/Lambda/API Gatewayが構築され、以後のAPI呼び出し・記事投入が可能になります。

手順:

```bash
# 合成（テンプレート生成）
cdklocal synth

# デプロイ
cdklocal deploy --require-approval never
```

完了後、出力ログに API Gateway の RestApiId が表示されます。必要に応じて `CfnOutput` で出力することも可能です。

CDK 側の出力例（Go）:

```go
awscdk.NewCfnOutput(stack, awsString("ApiEndpoint"), &awscdk.CfnOutputProps{
    Value: awsString(fmt.Sprintf("http://localhost:4566/restapis/%s/prod/_user_request_/", *api.RestApiId())),
})
```

API の URL 形式（LocalStack）:

```text
http://localhost:4566/restapis/{restApiId}/prod/_user_request_/posts
http://localhost:4566/restapis/{restApiId}/prod/_user_request_/posts/{id}
```

---

### 8. API 実行

- 目的: 現在の最小ハンドラでの動作確認（GET のみ）を行います。
- 注: 現状の Lambda はモック応答で S3 を参照しません。POST/PUT/DELETE の CRUD を行うには付録Aの実装に置き換えて再デプロイしてください。

手順（API・最小実装／GET のみ）:

```bash
# REST API ID の取得
REST_API_ID=$(awslocal apigateway get-rest-apis | jq -r '.items[0].id')
BASE="http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_"

# 1) 一覧（GET /posts）
curl -s "${BASE}/posts" | jq .

# 2) 1件取得（GET /posts/hello）
curl -s "${BASE}/posts/hello"
```

期待結果:

- GET /posts は配列（JSON）を返します（サンプル1件: `hello`）。
- GET /posts/{id} は markdown テキストを返します（例: `hello`）。

CRUD を有効化したい場合（付録Aを適用）:

1) `lambda/cmd/blog/main.go` を付録A「CRUD 対応 Lambda（完全版）」の実装に置き換えます。
2) Lambda を再ビルドして ZIP を更新し、再デプロイします。

```bash
mkdir -p dist/lambda/blog
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/lambda/blog/bootstrap lambda/cmd/blog
( cd dist/lambda/blog && zip -j ../blog.zip bootstrap )
cdklocal deploy --require-approval never
```

手順（S3 直接／CRUD 版のみ有効）:

```bash
# サンプル記事（JSON）
cat > hello.json <<'EOF'
{"id":"hello","title":"Hello","content":"# Hello from LocalStack\nThis is a sample."}
EOF

# デプロイ済みバケット名の特定
POSTS_BUCKET=$(awslocal s3 ls | awk '{print $3}' | grep -i blogposts | head -n1)
echo "$POSTS_BUCKET"

# S3 にアップロード（キーは posts/{id}.json）
awslocal s3 cp hello.json s3://$POSTS_BUCKET/posts/hello.json

# 確認
awslocal s3 ls s3://$POSTS_BUCKET/posts/
```

---

### 9. API 実行（CRUD 実装後）

- 目的: 一連の CRUD 操作が API で成功することを確認
- 実際に行うこと: APIの作成→一覧→取得→更新→削除→削除確認の順でHTTP呼び出しを行い、想定ステータスコード/レスポンスが返ることを検証します。
- 結果: 作成から削除までの一連のユーザ操作が成功し、API設計とS3連携が期待どおりに機能していることを保証できます。

手順:

```bash
# RestApiId の取得
REST_API_ID=$(awslocal apigateway get-rest-apis | jq -r '.items[0].id')
echo "$REST_API_ID"

# 1) 作成（POST /posts）
curl -s -X POST \
  -H "Content-Type: application/json" \
  -d '{"id":"hello","title":"Hello","content":"# Hello from API\nThis is markdown content."}' \
  "http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_/posts" | jq .

# 2) 一覧（GET /posts）
curl -s "http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_/posts" | jq .

# 3) 取得（GET /posts/hello）
curl -s "http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_/posts/hello" | jq .

# 4) 更新（PUT /posts/hello）
curl -s -X PUT \
  -H "Content-Type: application/json" \
  -d '{"title":"Hello (updated)","content":"# Updated\nNew content."}' \
  "http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_/posts/hello" | jq .

# 5) 削除（DELETE /posts/hello）
curl -s -X DELETE "http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_/posts/hello" -i | head -n1

# 6) 削除確認（GET /posts/hello は 404）
curl -s -o /dev/null -w "%{http_code}\n" "http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_/posts/hello"
```

期待結果:

- POST/PUT が 200 で作成/更新後の JSON を返却
- GET /posts は配列（JSON）を返却
- DELETE は 204 No Content（ヘッダのみ）
- 削除後の GET は 404

---

### 10. 運用（更新/ログ/破棄）

- 目的: 実装更新や確認、リソース破棄の方法を押さえる
- 実際に行うこと: コード変更後の再デプロイ、Lambdaのログ追跡、作成済みスタックの破棄といった日常運用タスクを実行します。
- 結果: 変更の反映、問題発生時の原因追跡、不要リソースのクリーンアップができ、ローカルの環境を健全に保てます。

手順:

```bash
# Lambda コード変更後の再デプロイ
cdklocal deploy --require-approval never

# Lambda 実行ログ（CloudWatch Logs エミュレーション）
awslocal logs describe-log-groups
awslocal logs tail "/aws/lambda/BlogApi" --follow

# スタック破棄（LocalStack 内の作成リソースを削除）
cdklocal destroy --force
```

---

### 11. トラブルシューティング

#### CDK デプロイ関連の問題

##### `cdklocal bootstrap` / `deploy` で失敗する

**症状**: bootstrap や deploy コマンドが失敗する

**原因と解決策**:

1. **LocalStack の起動確認**
   - 確認方法: `docker compose ps`
   - LocalStack コンテナが起動していない場合は `docker compose up -d` で起動

2. **環境変数の設定確認**
   - `AWS_DEFAULT_REGION` などの環境変数を再確認
   - 必要に応じて再設定: `export AWS_DEFAULT_REGION=ap-northeast-1`

3. **cdklocal が aws-cdk を見つけられない（MODULE_NOT_FOUND）**
   - **原因**: cdklocal は内部で `aws-cdk` のAPIを呼ぶため、同一NODE_PATH/依存に `aws-cdk` が必要
   - **解決策（推奨）**: プロジェクトへローカル導入して npx 経由で実行

     ```bash
     npm install -D aws-cdk aws-cdk-local
     # 実行は npx cdklocal <cmd> / npx cdk <cmd>
     ```

   - **代替（グローバル）**: `npm install -g aws-cdk aws-cdk-local` 後、 `export NODE_PATH=$(npm root -g)` を設定
   - **Homebrew 経由の aws-cdk を使用している場合**: `NODE_PATH` に brew のモジュールパスを追加

     ```bash
     export NODE_PATH="$(brew --prefix aws-cdk)/libexec/lib/node_modules:$NODE_PATH"
     ```

4. **SSM が無効で bootstrap が失敗する**
   - **症状**: `Service 'ssm' is not enabled. Please check your 'SERVICES' configuration variable.`
   - **解決策**: `docker-compose.yml` の `SERVICES` に `ssm, sts, ecr` を追加して LocalStack を再起動

5. **`panic: Cannot find asset at dist/lambda/blog.zip`**
   - **原因**: CDK アプリ内で `awslambda.Code_FromAsset("dist/lambda/blog.zip")` を使用しており、ZIP が未作成
   - **解決策**: 事前に Lambda をビルドして ZIP を作成

     ```bash
     mkdir -p dist/lambda/blog
     CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/lambda/blog/bootstrap lambda/cmd/blog
     ( cd dist/lambda/blog && zip -j ../blog.zip bootstrap )
     ```

##### cdklocal のインストール問題

**症状**: `cdklocal` が見つからない、またはインストールでエラーが発生

**原因と解決策**:

1. **PyPI からのインストール失敗**
   - **原因**: `aws-cdk-local` はPyPIに存在せず、 `pipx install` は失敗します（エラー: No matching distribution）
   - **解決策**: `npm install -g aws-cdk-local` でインストール、または `npx cdklocal <cmd>` を使用

2. **npx コマンドの誤用**
   - **症状**: `npx install -g aws-cdk-local` でエラー（could not determine executable to run）
   - **原因**: `npx install -g` はコマンドとして無効
   - **解決策**: `npm install -g aws-cdk-local` を用いてグローバル化するか、 `npx cdklocal <cmd>` を直接実行

3. **既存 CDK との競合**
   - **症状**: `npm install -g aws-cdk` で EEXIST（既存ファイルあり）
   - **確認**: 既に `cdk` コマンドが存在。 `cdk --version` が出ればインストールは不要
   - **解決策1**: `npm install -g aws-cdk --force` で上書き（注意）
   - **解決策2**: `npm uninstall -g aws-cdk && npm install -g aws-cdk`

##### CDK 初期化の問題

**症状**: `cdk init app --language go` が "cannot be run in a non-empty directory" で失敗

**原因**: CDKの初期化は空ディレクトリで行う必要があります

**解決策**: 新しいディレクトリを作成して移動してから実行

```bash
mkdir my-cdk-app && cd my-cdk-app && cdk init app --language go
```

#### Go 環境関連の問題

##### Go モジュール取得失敗

**症状**: unexpected EOF / proxyエラー等でモジュール取得に失敗

**原因**: ネットワークの一時的な断/タイムアウト、GOPROXY経由の取得失敗、バージョン解決の揺らぎ

**解決策**:

1. **リトライ**: `go clean -modcache && go mod tidy` または `go get -u <module>@<version>`
2. **バージョン固定**: `go get github.com/aws/aws-cdk-go/awscdk/v2@v2.219.0` 等で安定化
3. **プロキシ切替**: `export GOPROXY=https://proxy.golang.org,direct` を設定してから再試行
4. **一時的設定調整**: `GOPRIVATE` / `GONOSUMDB` を調整して検証（必要時）

##### awslambdagoalpha の取得問題

**症状**: `awslambdagoalpha` が取得できない / import解決できない

**原因**: v2配下以外のパス指定、または環境により `awslambdagoalpha` の取得が不安定

**解決策**:

- **代替手法**: `awslambda.Code.fromAsset` でビルド済みZIPを配布する方式に切り替える（本手順の実装へ更新済み）
- **補足**: ZIP方式はSDKや依存の揺らぎを避けやすく、CI上でも再現性が高い

##### Go CDK v1/v2 の混在による型エラー

**症状**: `constructs/v3` と `constructs/v10`、`awscdk v1` と `awscdk/v2` の混在でビルドエラー（例: "does not implement constructs.Construct"）

**解決策**:

- `go.mod` を `awscdk/v2` と `constructs/v10` に統一し、`constructs/v3` の間接参照を排除
- import を v2 配下に統一（例: `github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway`）
- 文字列は `jsii.String()` を使用、`GrantReadWrite` のシグネチャに合わせる（`bucket.GrantReadWrite(fn, nil)` など）

##### Lambda アーキテクチャ不一致

**症状**: 実行時にアーキテクチャ不一致でエラー（例: x86_64想定のランタイムに arm64 バイナリを配置）

**解決策**:

- 既定では Lambda のアーキテクチャは x86_64。`GOOS=linux GOARCH=amd64` でビルド
- もし `arm64` を使う場合は、CDK 側で `Architecture_ARM_64` を指定する

---

## 付録

この付録では、最小実装から実運用に近い形へ拡張するための具体コードと運用タスク（ビルド/デプロイ支援）を提供します。

- 何をしているか: LambdaのCRUD実装・LocalStack向けエンドポイント設定例・ビルド/ZIP化・cdklocal操作の定型タスクを提示
- これにより: 記事の作成/更新/削除を含むAPIを短時間で有効化でき、再現性の高いデプロイ/運用フロー（Makefile）で開発ループを高速化できます。

### A. CRUD 対応 Lambda（完全版）

- 何をしているか: 4章の最小ハンドラを、S3を読み書きする本格的なCRUDに差し替えます。API Gatewayからのリクエストを受け、S3バケット `POSTS_BUCKET` に `posts/{id}.json` を生成/更新/削除し、取得時はJSON（一覧/1件）を返します。
- これにより: ブログ記事をAPI経由でフルCRUD操作できるようになり、クライアントやCIから統一的にデータ管理が可能になります。

4章の雛形をこの実装に置き換えることで、最小 CRUD（POST/GET/PUT/DELETE）が有効になります。S3 に `posts/{id}.json` を保存します。

```go
// lambda/cmd/blog/main.go（CRUD対応）
package main

import (
 "bytes"
 "context"
 "encoding/json"
 "io"
 "net/http"
 "os"
 "fmt"
 "strings"

 "github.com/aws/aws-lambda-go/events"
 "github.com/aws/aws-lambda-go/lambda"
 "github.com/aws/aws-sdk-go-v2/config"
 "github.com/aws/aws-sdk-go-v2/service/s3"
)

type Post struct {
 ID      string `json:"id"`
 Title   string `json:"title"`
 Content string `json:"content"`
}

var (
 s3Client *s3.Client
 bucket   string
)

func init() {
 bucket = os.Getenv("POSTS_BUCKET")
 cfg, _ := config.LoadDefaultConfig(context.Background())
 s3Client = s3.NewFromConfig(cfg)
}

func handle(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
 path := req.Path
 method := req.HTTPMethod

 if method == http.MethodGet && path == "/posts" {
  // 一覧: posts/*.json を読み込んで配列にして返す
  prefix := "posts/"
  out, err := s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &bucket, Prefix: &prefix})
  if err != nil { return errorJSON(500, "list failed") }
  var posts []Post
  for _, obj := range out.Contents {
   key := *obj.Key
   if !strings.HasSuffix(key, ".json") { continue }
   po, err := s3Client.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
   if err != nil { continue }
   b, _ := io.ReadAll(po.Body)
   _ = po.Body.Close()
   var p Post
   if json.Unmarshal(b, &p) == nil { posts = append(posts, p) }
  }
  return jsonOK(posts), nil
 }

 if method == http.MethodGet && strings.HasPrefix(path, "/posts/") {
  id := strings.TrimPrefix(path, "/posts/")
  key := fmt.Sprintf("posts/%s.json", id)
  po, err := s3Client.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
  if err != nil { return errorJSON(404, "not found") }
  b, _ := io.ReadAll(po.Body)
  _ = po.Body.Close()
  return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(b), Headers: map[string]string{"Content-Type": "application/json"}}, nil
 }

 if method == http.MethodPost && path == "/posts" {
  var p Post
  if err := json.Unmarshal([]byte(req.Body), &p); err != nil || p.ID == "" {
   return errorJSON(400, "invalid body: require id,title,content")
  }
  key := fmt.Sprintf("posts/%s.json", p.ID)
  b, _ := json.Marshal(p)
  ct := "application/json"
  _, err := s3Client.PutObject(ctx, &s3.PutObjectInput{Bucket: &bucket, Key: &key, Body: bytes.NewReader(b), ContentType: &ct})
  if err != nil { return errorJSON(500, "create failed") }
  return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(b), Headers: map[string]string{"Content-Type": "application/json"}}, nil
 }

 if method == http.MethodPut && strings.HasPrefix(path, "/posts/") {
  id := strings.TrimPrefix(path, "/posts/")
  var p Post
  if err := json.Unmarshal([]byte(req.Body), &p); err != nil { return errorJSON(400, "invalid body") }
  p.ID = id
  key := fmt.Sprintf("posts/%s.json", id)
  b, _ := json.Marshal(p)
  ct := "application/json"
  _, err := s3Client.PutObject(ctx, &s3.PutObjectInput{Bucket: &bucket, Key: &key, Body: bytes.NewReader(b), ContentType: &ct})
  if err != nil { return errorJSON(500, "update failed") }
  return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(b), Headers: map[string]string{"Content-Type": "application/json"}}, nil
 }

 if method == http.MethodDelete && strings.HasPrefix(path, "/posts/") {
  id := strings.TrimPrefix(path, "/posts/")
  key := fmt.Sprintf("posts/%s.json", id)
  _, err := s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &bucket, Key: &key})
  if err != nil { return errorJSON(500, "delete failed") }
  return events.APIGatewayProxyResponse{StatusCode: 204, Body: ""}, nil
 }

 return errorJSON(404, "not found")
}

func jsonOK(v interface{}) events.APIGatewayProxyResponse {
 b, _ := json.Marshal(v)
 return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(b), Headers: map[string]string{"Content-Type": "application/json"}}
}

func errorJSON(code int, msg string) (events.APIGatewayProxyResponse, error) {
 b, _ := json.Marshal(map[string]string{"error": msg})
 return events.APIGatewayProxyResponse{StatusCode: code, Body: string(b), Headers: map[string]string{"Content-Type": "application/json"}}, nil
}

func main() { lambda.Start(handle) }
```

#### A-1. LocalStack エンドポイント設定（必要時）

- 何をしているか: Go SDK v2 のエンドポイント解決を上書きし、S3クライアントがLocalStackの `:4566` に向くようにします。またパススタイルを有効化して互換性を高めます。
- これにより: SDKの自動検出がうまく働かない環境でも、確実にLocalStackへ接続でき、S3操作の失敗（リージョン解決や署名先の不一致）を回避できます。

コード差分（initの差し替え例）:

```go
// import に追加:
//   aws "github.com/aws/aws-sdk-go-v2/aws"

func init() {
    bucket = os.Getenv("POSTS_BUCKET")

    endpoint := os.Getenv("AWS_ENDPOINT_URL")
    if endpoint == "" {
        // Docker Compose で localstack サービス名を解決
        endpoint = "http://localstack:4566"
    }

    resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...any) (aws.Endpoint, error) {
        return aws.Endpoint{URL: endpoint, PartitionID: "aws", SigningRegion: region}, nil
    })

    cfg, _ := config.LoadDefaultConfig(context.Background(), config.WithEndpointResolverWithOptions(resolver))
    s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = true })
}
```

注:

- LocalStack の Transparent endpoint injection により明示設定が不要な場合もありますが、SDKや環境の違いで失敗する場合に有効です（参考: LocalStack Docs `Transparent endpoint injection`）。

### B. Makefile（任意）

- 何をしているか: Lambdaのビルド（Linux/amd64用bootstrap生成→ZIP化）と、cdklocalの `bootstrap/synth/deploy/destroy/logs` を定型タスク化しています。
- これにより: ワンコマンドでビルド〜デプロイが行え、ヒューマンエラー（ZIP未作成・環境変数未設定など）の抑止と開発ループの短縮が可能です。

```makefile
SHELL := /bin/bash

# Go Lambda をビルドしてZIP化（Linux/amd64でビルド、bootstrap実行形式）
build-lambda:
 mkdir -p dist/lambda/blog
 CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/lambda/blog/bootstrap lambda/cmd/blog
 cd dist/lambda/blog && zip -j ../blog.zip bootstrap

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

### C. よくある構成拡張

- レンダリング層の追加（Markdown → HTML 変換）
- API レスポンスのキャッシュ（API GW キャッシュ / Lambda 内キャッシュ）
- 認証/認可（Cognito は LocalStack Pro 対応領域）

---

## 完了

この手順書に従うことで、CDK（Go）+ LocalStack で API Gateway + Lambda + S3 によるサーバーレスなブログ API をローカルで構築・デプロイ・検証できます（参考: <https://zenn.dev/okojomoeko/articles/4584312c51810d>, <https://zenn.dev/kin/articles/d22f9b30263afb>）。
