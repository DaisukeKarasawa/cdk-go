# インフラ改善

## 概要

CDKスタックを構造化し、本格的なプロダクション環境に対応できるインフラ構成に改善します。

**目的**: CDKスタックのモジュール化、環境別設定の分離、セキュリティ強化、モニタリング追加

**期待される効果**: 保守性の向上、環境管理の効率化、セキュリティの強化、運用監視の充実

**リスク**: インフラ変更によるサービス停止、設定の複雑化

## 現状分析

### Before（改善前の状態）

**CDKスタックの課題**:

- **単一ファイル**: 88行の`cdk-go.go`にすべてのリソースが定義
- **ハードコーディング**: 環境別設定ができない
- **セキュリティ不足**: IAM権限の最小化が不十分
- **モニタリングなし**: ログやメトリクスの収集が不足
- **タグ付けなし**: リソース管理に必要なタグが不足

**構成**:

```go
// 現在の構成（要約）
func NewCdkGoStack(scope constructs.Construct, id string, props *CdkGoStackProps) awscdk.Stack {
    // S3、Lambda、API Gatewayが1つの関数に混在
    bucket := awss3.NewBucket(stack, jsii.String("BlogPosts"), &awss3.BucketProps{})
    fn := awslambda.NewFunction(stack, jsii.String("BlogApi"), &awslambda.FunctionProps{...})
    api := awsapigateway.NewLambdaRestApi(stack, jsii.String("BlogApiGateway"), &awsapigateway.LambdaRestApiProps{...})
    return stack
}
```

## リファクタリング手順

### 1. CDKスタックの構造化

**目的**: 責務に応じたConstruct分離

**新しい構成**:

```
infrastructure/
├── constructs/
│   ├── blog-storage/          # S3バケット関連
│   │   └── blog-storage.go
│   ├── blog-api/              # API Gateway関連
│   │   └── blog-api.go
│   ├── blog-lambda/           # Lambda関数関連
│   │   └── blog-lambda.go
│   └── monitoring/            # モニタリング関連
│       └── monitoring.go
├── stacks/
│   ├── blog-stack.go          # メインスタック
│   └── monitoring-stack.go    # モニタリングスタック
└── config/
    ├── environment.go         # 環境設定
    └── tags.go               # タグ設定
```

### 2. 環境設定の分離

**目的**: 環境別設定の管理

**ファイル**: `infrastructure/config/environment.go`

```go
package config

import (
	"github.com/aws/jsii-runtime-go"
)

// Environment 環境設定
type Environment struct {
	Name        string
	Region      string
	Account     string
	Environment string
	Tags        map[string]string
}

// GetEnvironment 環境設定を取得
func GetEnvironment() *Environment {
	env := os.Getenv("CDK_ENVIRONMENT")
	if env == "" {
		env = "dev"
	}

	region := os.Getenv("CDK_DEFAULT_REGION")
	if region == "" {
		region = "ap-northeast-1"
	}

	account := os.Getenv("CDK_DEFAULT_ACCOUNT")
	if account == "" {
		account = "000000000000" // LocalStack用
	}

	return &Environment{
		Name:        fmt.Sprintf("blog-%s", env),
		Region:      region,
		Account:     account,
		Environment: env,
		Tags: map[string]string{
			"Environment": env,
			"Project":     "blog-api",
			"ManagedBy":   "cdk",
		},
	}
}

// GetBucketName バケット名を取得
func (e *Environment) GetBucketName() string {
	return fmt.Sprintf("%s-posts-%s", e.Name, e.Region)
}

// GetLambdaFunctionName Lambda関数名を取得
func (e *Environment) GetLambdaFunctionName() string {
	return fmt.Sprintf("%s-api", e.Name)
}

// GetAPIName API名を取得
func (e *Environment) GetAPIName() string {
	return fmt.Sprintf("%s-gateway", e.Name)
}
```

**ファイル**: `infrastructure/config/tags.go`

```go
package config

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
)

// ApplyTags リソースにタグを適用
func ApplyTags(scope constructs.Construct, env *Environment) {
	awscdk.Tags_Of(scope).Add(jsii.String("Environment"), jsii.String(env.Environment), nil)
	awscdk.Tags_Of(scope).Add(jsii.String("Project"), jsii.String("blog-api"), nil)
	awscdk.Tags_Of(scope).Add(jsii.String("ManagedBy"), jsii.String("cdk"), nil)
	awscdk.Tags_Of(scope).Add(jsii.String("CostCenter"), jsii.String("engineering"), nil)
}
```

### 3. ストレージConstructの実装

**目的**: S3バケットの独立化

**ファイル**: `infrastructure/constructs/blog-storage/blog-storage.go`

```go
package blogstorage

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"

	"infrastructure/config"
)

// BlogStorageProps ストレージConstructのプロパティ
type BlogStorageProps struct {
	Environment *config.Environment
}

// BlogStorage ブログストレージConstruct
type BlogStorage struct {
	Bucket awss3.Bucket
}

// NewBlogStorage ストレージConstructのコンストラクタ
func NewBlogStorage(scope constructs.Construct, id string, props *BlogStorageProps) *BlogStorage {
	construct := constructs.NewConstruct(scope, &id)

	// S3バケットの作成
	bucket := awss3.NewBucket(construct, jsii.String("PostsBucket"), &awss3.BucketProps{
		BucketName: jsii.String(props.Environment.GetBucketName()),

		// セキュリティ設定
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		Encryption:        awss3.BucketEncryption_S3_MANAGED(),

		// ライフサイクル設定
		LifecycleRules: &[]*awss3.LifecycleRule{
			{
				Id:     jsii.String("DeleteOldVersions"),
				Status: awss3.LifecycleRuleStatus_ENABLED(),
				NoncurrentVersionExpiration: awscdk.Duration_Days(jsii.Number(30)),
			},
		},

		// バージョニング
		Versioned: jsii.Bool(true),

		// ログ設定
		ServerAccessLogsBucket: awss3.NewBucket(construct, jsii.String("AccessLogsBucket"), &awss3.BucketProps{
			BucketName: jsii.String(fmt.Sprintf("%s-access-logs", props.Environment.GetBucketName())),
		}),
	})

	// タグの適用
	config.ApplyTags(construct, props.Environment)

	return &BlogStorage{
		Bucket: bucket,
	}
}

// GrantReadWrite 読み書き権限を付与
func (s *BlogStorage) GrantReadWrite(construct constructs.Construct) {
	s.Bucket.GrantReadWrite(construct, nil)
}

// GetBucketName バケット名を取得
func (s *BlogStorage) GetBucketName() *string {
	return s.Bucket.BucketName()
}
```

### 4. Lambda Constructの実装

**目的**: Lambda関数の独立化

**ファイル**: `infrastructure/constructs/blog-lambda/blog-lambda.go`

```go
package bloglambda

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"

	"infrastructure/config"
)

// BlogLambdaProps Lambda Constructのプロパティ
type BlogLambdaProps struct {
	Environment    *config.Environment
	PostsBucket    awss3.Bucket
	Code           awslambda.Code
}

// BlogLambda ブログLambda Construct
type BlogLambda struct {
	Function awslambda.Function
}

// NewBlogLambda Lambda Constructのコンストラクタ
func NewBlogLambda(scope constructs.Construct, id string, props *BlogLambdaProps) *BlogLambda {
	construct := constructs.NewConstruct(scope, &id)

	// ロググループの作成
	logGroup := awslogs.NewLogGroup(construct, jsii.String("LogGroup"), &awslogs.LogGroupProps{
		LogGroupName:  jsii.String(fmt.Sprintf("/aws/lambda/%s", props.Environment.GetLambdaFunctionName())),
		RetentionDays: awslogs.RetentionDays_ONE_WEEK(),
	})

	// Lambda関数の作成
	function := awslambda.NewFunction(construct, jsii.String("BlogApiFunction"), &awslambda.FunctionProps{
		FunctionName: jsii.String(props.Environment.GetLambdaFunctionName()),
		Runtime:      awslambda.Runtime_PROVIDED_AL2(),
		Handler:      jsii.String("bootstrap"),
		Code:         props.Code,

		// パフォーマンス設定
		MemorySize:   jsii.Number(256),
		Timeout:      awscdk.Duration_Seconds(jsii.Number(30)),

		// 環境変数
		Environment: &map[string]*string{
			"POSTS_BUCKET": props.PostsBucket.BucketName(),
			"LOG_LEVEL":    jsii.String("INFO"),
			"ENVIRONMENT":  jsii.String(props.Environment.Environment),
		},

		// ログ設定
		LogGroup: logGroup,

		// デッドレターキュー
		DeadLetterQueue: awssqs.NewQueue(construct, jsii.String("DLQ"), &awssqs.QueueProps{
			QueueName: jsii.String(fmt.Sprintf("%s-dlq", props.Environment.GetLambdaFunctionName())),
		}),

		// 並行実行設定
		ReservedConcurrentExecutions: jsii.Number(10),

		// トレーシング
		Tracing: awslambda.Tracing_ACTIVE(),
	})

	// S3バケットへの権限付与
	props.PostsBucket.GrantReadWrite(function, nil)

	// タグの適用
	config.ApplyTags(construct, props.Environment)

	return &BlogLambda{
		Function: function,
	}
}

// GetFunction Lambda関数を取得
func (l *BlogLambda) GetFunction() awslambda.Function {
	return l.Function
}
```

### 5. API Gateway Constructの実装

**目的**: API Gatewayの独立化

**ファイル**: `infrastructure/constructs/blog-api/blog-api.go`

```go
package blogapi

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"

	"infrastructure/config"
)

// BlogAPIProps API Gateway Constructのプロパティ
type BlogAPIProps struct {
	Environment *config.Environment
	LambdaFunction awslambda.Function
}

// BlogAPI ブログAPI Construct
type BlogAPI struct {
	RestAPI awsapigateway.RestApi
}

// NewBlogAPI API Gateway Constructのコンストラクタ
func NewBlogAPI(scope constructs.Construct, id string, props *BlogAPIProps) *BlogAPI {
	construct := constructs.NewConstruct(scope, &id)

	// API Gatewayの作成
	restAPI := awsapigateway.NewRestApi(construct, jsii.String("BlogRestAPI"), &awsapigateway.RestApiProps{
		RestApiName: jsii.String(props.Environment.GetAPIName()),

		// CORS設定
		DefaultCorsPreflightOptions: &awsapigateway.CorsOptions{
			AllowOrigins:  awsapigateway.Cors_ALL_ORIGINS(),
			AllowMethods:  awsapigateway.Cors_ALL_METHODS(),
			AllowHeaders:  awsapigateway.Cors_DEFAULT_HEADERS(),
			MaxAge:        awscdk.Duration_Days(jsii.Number(1)),
		},

		// デプロイメント設定
		DeployOptions: &awsapigateway.StageOptions{
			StageName: jsii.String("prod"),
			ThrottleRateLimit:  jsii.Number(1000),
			ThrottleBurstLimit: jsii.Number(2000),
		},

		// エンドポイント設定
		EndpointConfiguration: &awsapigateway.EndpointConfiguration{
			Types: &[]awsapigateway.EndpointType{
				awsapigateway.EndpointType_REGIONAL(),
			},
		},
	})

	// Lambda統合
	lambdaIntegration := awsapigateway.NewLambdaIntegration(props.LambdaFunction, &awsapigateway.LambdaIntegrationOptions{
		ProxyIntegration: jsii.Bool(true),
	})

	// リソースとメソッドの定義
	posts := restAPI.Root().AddResource(jsii.String("posts"), nil)
	posts.AddMethod(jsii.String("GET"), lambdaIntegration, nil)
	posts.AddMethod(jsii.String("POST"), lambdaIntegration, nil)

	post := posts.AddResource(jsii.String("{id}"), nil)
	post.AddMethod(jsii.String("GET"), lambdaIntegration, nil)
	post.AddMethod(jsii.String("PUT"), lambdaIntegration, nil)
	post.AddMethod(jsii.String("DELETE"), lambdaIntegration, nil)

	// タグの適用
	config.ApplyTags(construct, props.Environment)

	return &BlogAPI{
		RestAPI: restAPI,
	}
}

// GetRestAPI REST APIを取得
func (a *BlogAPI) GetRestAPI() awsapigateway.RestApi {
	return a.RestAPI
}

// GetURL API URLを取得
func (a *BlogAPI) GetURL() *string {
	return a.RestAPI.Url()
}
```

### 6. モニタリングConstructの実装

**目的**: 監視・アラート機能の追加

**ファイル**: `infrastructure/constructs/monitoring/monitoring.go`

```go
package monitoring

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatchactions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"

	"infrastructure/config"
)

// MonitoringProps モニタリングConstructのプロパティ
type MonitoringProps struct {
	Environment     *config.Environment
	LambdaFunction  awslambda.Function
	RestAPI         awsapigateway.RestApi
}

// Monitoring モニタリングConstruct
type Monitoring struct {
	AlarmTopic awssns.Topic
}

// NewMonitoring モニタリングConstructのコンストラクタ
func NewMonitoring(scope constructs.Construct, id string, props *MonitoringProps) *Monitoring {
	construct := constructs.NewConstruct(scope, &id)

	// SNSトピックの作成
	alarmTopic := awssns.NewTopic(construct, jsii.String("AlarmTopic"), &awssns.TopicProps{
		TopicName: jsii.String(fmt.Sprintf("%s-alarms", props.Environment.Name)),
		DisplayName: jsii.String("Blog API Alarms"),
	})

	// Lambda関数のアラーム
	lambdaErrorAlarm := awscloudwatch.NewAlarm(construct, jsii.String("LambdaErrorAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:        jsii.String(fmt.Sprintf("%s-lambda-errors", props.Environment.Name)),
		AlarmDescription: jsii.String("Lambda function errors"),
		Metric: props.LambdaFunction.MetricErrors(&awslambda.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
		Threshold:         jsii.Number(5),
		EvaluationPeriods: jsii.Number(2),
		TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING(),
	})

	lambdaDurationAlarm := awscloudwatch.NewAlarm(construct, jsii.String("LambdaDurationAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:        jsii.String(fmt.Sprintf("%s-lambda-duration", props.Environment.Name)),
		AlarmDescription: jsii.String("Lambda function duration"),
		Metric: props.LambdaFunction.MetricDuration(&awslambda.MetricOptions{
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
		Threshold:         jsii.Number(10000), // 10秒
		EvaluationPeriods: jsii.Number(2),
		TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING(),
	})

	// API Gatewayのアラーム
	apiErrorAlarm := awscloudwatch.NewAlarm(construct, jsii.String("APIErrorAlarm"), &awscloudwatch.AlarmProps{
		AlarmName:        jsii.String(fmt.Sprintf("%s-api-errors", props.Environment.Name)),
		AlarmDescription: jsii.String("API Gateway errors"),
		Metric: awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/ApiGateway"),
			MetricName: jsii.String("4XXError"),
			DimensionsMap: &map[string]*string{
				"ApiName": props.RestAPI.RestApiName(),
			},
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
		Threshold:         jsii.Number(10),
		EvaluationPeriods: jsii.Number(2),
		TreatMissingData:  awscloudwatch.TreatMissingData_NOT_BREACHING(),
	})

	// アラームをSNSトピックに送信
	lambdaErrorAlarm.AddAlarmAction(awscloudwatchactions.NewSnsAction(alarmTopic))
	lambdaDurationAlarm.AddAlarmAction(awscloudwatchactions.NewSnsAction(alarmTopic))
	apiErrorAlarm.AddAlarmAction(awscloudwatchactions.NewSnsAction(alarmTopic))

	// ダッシュボードの作成
	dashboard := awscloudwatch.NewDashboard(construct, jsii.String("BlogDashboard"), &awscloudwatch.DashboardProps{
		DashboardName: jsii.String(fmt.Sprintf("%s-dashboard", props.Environment.Name)),
	})

	// ダッシュボードにウィジェットを追加
	dashboard.AddWidgets(
		awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
			Title: jsii.String("Lambda Invocations"),
			Left: &[]awscloudwatch.IMetric{
				props.LambdaFunction.MetricInvocations(&awslambda.MetricOptions{
					Period: awscdk.Duration_Minutes(jsii.Number(5)),
				}),
			},
		}),
		awscloudwatch.NewGraphWidget(&awscloudwatch.GraphWidgetProps{
			Title: jsii.String("Lambda Duration"),
			Left: &[]awscloudwatch.IMetric{
				props.LambdaFunction.MetricDuration(&awslambda.MetricOptions{
					Period: awscdk.Duration_Minutes(jsii.Number(5)),
				}),
			},
		}),
	)

	// タグの適用
	config.ApplyTags(construct, props.Environment)

	return &Monitoring{
		AlarmTopic: alarmTopic,
	}
}

// GetAlarmTopic アラームトピックを取得
func (m *Monitoring) GetAlarmTopic() awssns.Topic {
	return m.AlarmTopic
}
```

### 7. メインスタックの実装

**目的**: 各Constructを統合

**ファイル**: `infrastructure/stacks/blog-stack.go`

```go
package stacks

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"

	"infrastructure/config"
	"infrastructure/constructs/blogapi"
	"infrastructure/constructs/bloglambda"
	"infrastructure/constructs/blogstorage"
	"infrastructure/constructs/monitoring"
)

// BlogStackProps ブログスタックのプロパティ
type BlogStackProps struct {
	awscdk.StackProps
	Environment *config.Environment
}

// BlogStack ブログスタック
type BlogStack struct {
	awscdk.Stack
	Storage    *blogstorage.BlogStorage
	Lambda     *bloglambda.BlogLambda
	API        *blogapi.BlogAPI
	Monitoring *monitoring.Monitoring
}

// NewBlogStack ブログスタックのコンストラクタ
func NewBlogStack(scope constructs.Construct, id string, props *BlogStackProps) *BlogStack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// 環境設定
	env := props.Environment
	if env == nil {
		env = config.GetEnvironment()
	}

	// ストレージの作成
	storage := blogstorage.NewBlogStorage(stack, "BlogStorage", &blogstorage.BlogStorageProps{
		Environment: env,
	})

	// Lambda関数の作成
	lambdaCode := awslambda.Code_FromAsset(jsii.String("dist/blog.zip"), nil)
	lambda := bloglambda.NewBlogLambda(stack, "BlogLambda", &bloglambda.BlogLambdaProps{
		Environment: env,
		PostsBucket: storage.Bucket,
		Code:        lambdaCode,
	})

	// API Gatewayの作成
	api := blogapi.NewBlogAPI(stack, "BlogAPI", &blogapi.BlogAPIProps{
		Environment:    env,
		LambdaFunction: lambda.Function,
	})

	// モニタリングの作成
	monitoring := monitoring.NewMonitoring(stack, "Monitoring", &monitoring.MonitoringProps{
		Environment:    env,
		LambdaFunction: lambda.Function,
		RestAPI:        api.RestAPI,
	})

	// 出力の設定
	awscdk.NewCfnOutput(stack, jsii.String("ApiEndpoint"), &awscdk.CfnOutputProps{
		Value:       api.GetURL(),
		Description: jsii.String("Blog API Endpoint"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("BucketName"), &awscdk.CfnOutputProps{
		Value:       storage.GetBucketName(),
		Description: jsii.String("Posts Bucket Name"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("AlarmTopicArn"), &awscdk.CfnOutputProps{
		Value:       monitoring.GetAlarmTopic().TopicArn(),
		Description: jsii.String("Alarm Topic ARN"),
	})

	return &BlogStack{
		Stack:      stack,
		Storage:    storage,
		Lambda:     lambda,
		API:        api,
		Monitoring: monitoring,
	}
}
```

### 8. メインアプリケーションの更新

**目的**: 新しいスタック構成の統合

**ファイル**: `cdk-go.go`（更新）

```go
package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"

	"infrastructure/config"
	"infrastructure/stacks"
)

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	// 環境設定
	environment := config.GetEnvironment()

	// ブログスタックの作成
	stacks.NewBlogStack(app, "BlogStack", &stacks.BlogStackProps{
		awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String(environment.Account),
				Region:  jsii.String(environment.Region),
			},
		},
		Environment: environment,
	})

	app.Synth(nil)
}
```

### 9. 環境別設定ファイル

**目的**: 環境別の設定管理

**ファイル**: `config/dev.json`

```json
{
  "environment": "dev",
  "region": "ap-northeast-1",
  "account": "000000000000",
  "lambda": {
    "memorySize": 256,
    "timeout": 30
  },
  "api": {
    "throttleRateLimit": 1000,
    "throttleBurstLimit": 2000
  },
  "monitoring": {
    "alarmThresholds": {
      "lambdaErrors": 5,
      "lambdaDuration": 10000,
      "apiErrors": 10
    }
  }
}
```

**ファイル**: `config/prod.json`

```json
{
  "environment": "prod",
  "region": "ap-northeast-1",
  "account": "123456789012",
  "lambda": {
    "memorySize": 512,
    "timeout": 60
  },
  "api": {
    "throttleRateLimit": 5000,
    "throttleBurstLimit": 10000
  },
  "monitoring": {
    "alarmThresholds": {
      "lambdaErrors": 1,
      "lambdaDuration": 5000,
      "apiErrors": 5
    }
  }
}
```

## 動作確認

### スタックの合成

```bash
# 環境変数の設定
export CDK_ENVIRONMENT=dev
export CDK_DEFAULT_REGION=ap-northeast-1
export CDK_DEFAULT_ACCOUNT=000000000000

# スタックの合成
cdklocal synth

# 生成されたテンプレートの確認
ls -la cdk.out/
```

### デプロイと確認

```bash
# デプロイ
cdklocal deploy --require-approval never

# リソースの確認
awslocal s3 ls
awslocal lambda list-functions
awslocal apigateway get-rest-apis
```

### 期待結果

- **モジュール化**: 各Constructが独立して管理
- **環境分離**: 環境別設定が適用
- **セキュリティ**: IAM権限の最小化
- **モニタリング**: アラームとダッシュボードが設定
- **タグ付け**: リソース管理用のタグが適用

## トラブルシューティング

### スタック合成エラー

**症状**: `cdklocal synth`でエラー

**原因**: 新しいConstructのimportエラー

**解決策**:

```bash
# 依存関係の確認
go mod tidy

# パッケージパスの確認
go list ./infrastructure/...
```

### デプロイエラー

**症状**: リソース作成でエラー

**原因**: 環境変数の設定不備

**解決策**:

```bash
# 環境変数の確認
echo $CDK_ENVIRONMENT
echo $CDK_DEFAULT_REGION
echo $CDK_DEFAULT_ACCOUNT
```

### 権限エラー

**症状**: Lambda関数でS3アクセスエラー

**原因**: IAM権限の設定不備

**解決策**:

```go
// 権限の明示的な付与
props.PostsBucket.GrantReadWrite(function, nil)
```

## 次のステップ

インフラ改善が完了したら、[デプロイとバリデーション](../refactoring/06-deployment.md)に進んでください。

**完了確認**:

- [ ] CDKスタックがモジュール化されている
- [ ] 環境別設定が分離されている
- [ ] セキュリティが強化されている
- [ ] モニタリングが実装されている
- [ ] タグ付けが適用されている

---

> **💡 ヒント**: インフラ改善は段階的に進めることが重要です。まず基本構造から始めて、徐々にセキュリティとモニタリングを追加してください。
