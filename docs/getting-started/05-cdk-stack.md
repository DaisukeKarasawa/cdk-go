# CDK スタック実装

## 概要

S3（記事格納用）、Lambda（API）、API Gateway（公開）を CDK（Go）で定義します。

**目的**: 記事データの保存先（S3）と、それにアクセスする実行関数（Lambda）、外部公開のHTTP入口（API Gateway）が1つのスタックとして連携

**リスク**: Goバイナリのクロスコンパイル設定やアセット配置ミス

## 依存関係の追加

```bash
# Option A: latest（通信環境により失敗する場合あり）
go get github.com/aws/aws-cdk-go/awscdk/v2@latest

# Option B: バージョン固定（推奨: ネットワーク起因の揺らぎ回避）
# 例）v2.219.0 に固定（必要に応じて調整してください）
# go get github.com/aws/aws-cdk-go/awscdk/v2@v2.219.0

go get github.com/aws/constructs-go/constructs/v10@latest
```

取得に失敗する場合は[トラブルシューティング](../guides/troubleshooting.md#go-モジュール取得失敗)を参照してください。

## スタック実装

プロジェクト生成時のスタックファイル（例: `cdk_go_stack.go`）を以下のように実装：

```go
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
		Code:    awslambda.Code_FromAsset(awsString("dist/blog.zip"), nil),
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

## 実装のポイント

### 1. S3 バケット

```go
bucket := awss3.NewBucket(stack, awsString("BlogPosts"), &awss3.BucketProps{})
```

- ブログ記事データ（JSON）を格納するバケット
- バケット名は自動生成（CDKが一意な名前を付与）

### 2. Lambda 関数

```go
fn := awslambda.NewFunction(stack, awsString("BlogApi"), &awslambda.FunctionProps{
	Runtime: awslambda.Runtime_PROVIDED_AL2(),
	Handler: awsString("bootstrap"),
	Code:    awslambda.Code_FromAsset(awsString("dist/blog.zip"), nil),
	Environment: &map[string]*string{
		"POSTS_BUCKET": bucket.BucketName(),
	},
})
```

- **Runtime**: `PROVIDED_AL2` - Go用のカスタムランタイム
- **Handler**: `bootstrap` - Go実行ファイル名
- **Code**: 事前ビルドしたZIPアセット（`dist/blog.zip`）を参照
- **Environment**: S3バケット名を環境変数で渡す

### 3. IAM 権限

```go
bucket.GrantReadWrite(fn, nil)
```

- Lambda関数にS3バケットの読み書き権限を付与
- CDKが適切なIAMポリシーを自動生成

### 4. API Gateway

```go
api := awsapigateway.NewLambdaRestApi(stack, awsString("BlogApiGateway"), &awsapigateway.LambdaRestApiProps{
	Handler: fn,
})
```

- `LambdaRestApi` - Lambda統合のREST API
- すべてのHTTPリクエストがLambda関数にプロキシされる
- API ルーティングは Lambda 側の `APIGatewayProxyRequest.Path` で分岐

## オプション: エンドポイント出力

デプロイ後にAPI URLを出力したい場合：

```go
awscdk.NewCfnOutput(stack, awsString("ApiEndpoint"), &awscdk.CfnOutputProps{
	Value: awsString(fmt.Sprintf("http://localhost:4566/restapis/%s/prod/_user_request_/", *api.RestApiId())),
})
```

## 事前準備の確認

スタック実装前に、Lambda ZIPが作成されていることを確認：

```bash
# Lambda ビルド & ZIP作成
mkdir -p dist/blog
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/blog/bootstrap lambda/cmd/blog
( cd dist/blog && zip -j ../blog.zip bootstrap )

# ファイル確認
ls -la dist/blog.zip
```

## 次のステップ

CDKスタックの実装が完了したら、[デプロイ](./06-deployment.md)に進んでください。
