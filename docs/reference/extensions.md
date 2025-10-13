# 拡張ガイド

## 概要

基本的なブログAPIから、より実用的なアプリケーションへの拡張方法を説明します。

## よくある構成拡張

### 1. レンダリング層の追加

#### Markdown → HTML 変換

**目的**: ブログ記事のMarkdownコンテンツをHTMLに変換して配信

**実装例**:

```go
import (
    "github.com/russross/blackfriday/v2"
)

// GET /posts/{id}/html エンドポイントを追加
if method == http.MethodGet && strings.Contains(path, "/html") {
    // posts/1/html -> posts/1.json を取得してHTML変換
    idStr := strings.Split(strings.TrimPrefix(path, "/posts/"), "/")[0]
    id, err := strconv.Atoi(idStr)
    if err != nil {
        return errorJSON(400, "invalid id")
    }

    // S3からMarkdownを取得
    key := fmt.Sprintf("posts/%d.json", id)
    po, err := s3Client.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
    if err != nil {
        return errorJSON(404, "not found")
    }

    var post Post
    b, _ := io.ReadAll(po.Body)
    po.Body.Close()
    json.Unmarshal(b, &post)

    // Markdown → HTML 変換
    html := blackfriday.Run([]byte(post.Content))

    return events.APIGatewayProxyResponse{
        StatusCode: 200,
        Body:       string(html),
        Headers:    map[string]string{"Content-Type": "text/html; charset=utf-8"},
    }, nil
}
```

**依存関係**:

```bash
go get github.com/russross/blackfriday/v2
```

#### テンプレートエンジン統合

**目的**: HTMLテンプレートを使用したページ生成

**実装例**:

```go
import (
    "html/template"
)

const htmlTemplate = `
<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
    <meta charset="utf-8">
</head>
<body>
    <h1>{{.Title}}</h1>
    <div>{{.HTMLContent}}</div>
</body>
</html>
`

type TemplateData struct {
    Title       string
    HTMLContent template.HTML
}

// テンプレート適用
tmpl := template.Must(template.New("post").Parse(htmlTemplate))
data := TemplateData{
    Title:       post.Title,
    HTMLContent: template.HTML(html),
}

var buf bytes.Buffer
tmpl.Execute(&buf, data)

return events.APIGatewayProxyResponse{
    StatusCode: 200,
    Body:       buf.String(),
    Headers:    map[string]string{"Content-Type": "text/html; charset=utf-8"},
}, nil
```

### 2. API レスポンスのキャッシュ

#### API Gateway キャッシュ

**目的**: 頻繁にアクセスされる記事のレスポンスをキャッシュして性能向上

**CDK実装**:

```go
api := awsapigateway.NewLambdaRestApi(stack, awsString("BlogApiGateway"), &awsapigateway.LambdaRestApiProps{
    Handler: fn,
    DefaultCorsPreflightOptions: &awsapigateway.CorsOptions{
        AllowOrigins: awsapigateway.Cors_ALL_ORIGINS(),
        AllowMethods: awsapigateway.Cors_ALL_METHODS(),
    },
})

// キャッシュ設定（LocalStackでは制限あり）
deployment := api.LatestDeployment()
stage := awsapigateway.NewStage(stack, awsString("ProdStage"), &awsapigateway.StageProps{
    Deployment: deployment,
    StageName:  awsString("prod"),
    CachingEnabled: jsii.Bool(true),
    CacheTtl: awscdk.Duration_Minutes(jsii.Number(5)),
})
```

#### Lambda 内キャッシュ

**目的**: Lambda関数内でメモリキャッシュを実装

**実装例**:

```go
import (
    "sync"
    "time"
)

type CacheItem struct {
    Data      []byte
    ExpiresAt time.Time
}

var (
    cache = make(map[string]CacheItem)
    mutex = sync.RWMutex{}
)

func getCachedPost(id int) ([]byte, bool) {
    mutex.RLock()
    defer mutex.RUnlock()

    key := fmt.Sprintf("post_%d", id)
    item, exists := cache[key]
    if !exists || time.Now().After(item.ExpiresAt) {
        return nil, false
    }
    return item.Data, true
}

func setCachedPost(id int, data []byte, ttl time.Duration) {
    mutex.Lock()
    defer mutex.Unlock()

    key := fmt.Sprintf("post_%d", id)
    cache[key] = CacheItem{
        Data:      data,
        ExpiresAt: time.Now().Add(ttl),
    }
}

// GET /posts/{id} でキャッシュを使用
if cached, found := getCachedPost(id); found {
    return events.APIGatewayProxyResponse{
        StatusCode: 200,
        Body:       string(cached),
        Headers:    map[string]string{"Content-Type": "application/json"},
    }, nil
}

// S3から取得してキャッシュに保存
// ... S3取得処理 ...
setCachedPost(id, b, 5*time.Minute)
```

### 3. 認証・認可

#### API Key 認証

**目的**: 簡易的なAPI認証を実装

**CDK実装**:

```go
// API Key の作成
apiKey := awsapigateway.NewApiKey(stack, awsString("BlogApiKey"), &awsapigateway.ApiKeyProps{
    ApiKeyName: awsString("blog-api-key"),
})

// Usage Plan の作成
usagePlan := awsapigateway.NewUsagePlan(stack, awsString("BlogUsagePlan"), &awsapigateway.UsagePlanProps{
    Name: awsString("blog-usage-plan"),
    Throttle: &awsapigateway.ThrottleSettings{
        RateLimit:  jsii.Number(100),
        BurstLimit: jsii.Number(200),
    },
})

// API Key と Usage Plan の関連付け
usagePlan.AddApiKey(apiKey, nil)
usagePlan.AddApiStage(&awsapigateway.UsagePlanPerApiStage{
    Api:   api,
    Stage: api.DeploymentStage(),
})
```

**Lambda での検証**:

```go
func validateApiKey(req events.APIGatewayProxyRequest) bool {
    apiKey := req.Headers["x-api-key"]
    expectedKey := os.Getenv("API_KEY")
    return apiKey == expectedKey
}

// ハンドラーの最初で認証チェック
if !validateApiKey(req) {
    return errorJSON(401, "unauthorized")
}
```

#### JWT 認証

**目的**: より高度な認証システムの実装

**実装例**:

```go
import (
    "github.com/golang-jwt/jwt/v5"
)

func validateJWT(tokenString string) (*jwt.Token, error) {
    secret := os.Getenv("JWT_SECRET")
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method")
        }
        return []byte(secret), nil
    })
    return token, err
}

// Authorization ヘッダーから JWT を取得・検証
authHeader := req.Headers["authorization"]
if !strings.HasPrefix(authHeader, "Bearer ") {
    return errorJSON(401, "missing bearer token")
}

tokenString := strings.TrimPrefix(authHeader, "Bearer ")
token, err := validateJWT(tokenString)
if err != nil || !token.Valid {
    return errorJSON(401, "invalid token")
}
```

### 4. データベース統合

#### DynamoDB 統合

**目的**: S3の代わりにDynamoDBを使用した高性能なデータ管理

**CDK実装**:

```go
// DynamoDB テーブル
table := awsdynamodb.NewTable(stack, awsString("BlogPosts"), &awsdynamodb.TableProps{
    TableName: awsString("blog-posts"),
    PartitionKey: &awsdynamodb.Attribute{
        Name: awsString("id"),
        Type: awsdynamodb.AttributeType_NUMBER,
    },
    BillingMode: awsdynamodb.BillingMode_PAY_PER_REQUEST,
})

// Lambda に DynamoDB 権限を付与
table.GrantReadWriteData(fn)
```

**Lambda実装**:

```go
import (
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var dynamoClient *dynamodb.Client

func init() {
    cfg, _ := config.LoadDefaultConfig(context.Background())
    dynamoClient = dynamodb.NewFromConfig(cfg)
}

// DynamoDB への保存
func savePostToDynamoDB(ctx context.Context, post Post) error {
    tableName := os.Getenv("TABLE_NAME")
    _, err := dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
        TableName: &tableName,
        Item: map[string]types.AttributeValue{
            "id":      &types.AttributeValueMemberN{Value: strconv.Itoa(post.ID)},
            "title":   &types.AttributeValueMemberS{Value: post.Title},
            "content": &types.AttributeValueMemberS{Value: post.Content},
        },
    })
    return err
}
```

### 5. 検索機能

#### 全文検索

**目的**: 記事のタイトルや内容での検索機能

**実装例**:

```go
// GET /posts/search?q=keyword
if method == http.MethodGet && path == "/posts/search" {
    query := req.QueryStringParameters["q"]
    if query == "" {
        return errorJSON(400, "missing query parameter")
    }

    // S3から全記事を取得して検索
    prefix := "posts/"
    out, err := s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
        Bucket: &bucket,
        Prefix: &prefix,
    })
    if err != nil {
        return errorJSON(500, "search failed")
    }

    var results []Post
    for _, obj := range out.Contents {
        // 各記事を取得して検索
        po, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
            Bucket: &bucket,
            Key:    obj.Key,
        })
        if err != nil {
            continue
        }

        var post Post
        b, _ := io.ReadAll(po.Body)
        po.Body.Close()
        json.Unmarshal(b, &post)

        // 簡易検索（タイトルと内容）
        if strings.Contains(strings.ToLower(post.Title), strings.ToLower(query)) ||
           strings.Contains(strings.ToLower(post.Content), strings.ToLower(query)) {
            results = append(results, post)
        }
    }

    return jsonOK(results), nil
}
```

### 6. ファイルアップロード

#### 画像アップロード

**目的**: ブログ記事に画像を添付する機能

**CDK実装**:

```go
// 画像用S3バケット
imagesBucket := awss3.NewBucket(stack, awsString("BlogImages"), &awss3.BucketProps{
    PublicReadAccess: jsii.Bool(true), // 画像の公開読み取り
})
imagesBucket.GrantReadWrite(fn, nil)
```

**Lambda実装**:

```go
// POST /posts/{id}/images
if method == http.MethodPost && strings.Contains(path, "/images") {
    // Base64エンコードされた画像データを受信
    var uploadReq struct {
        Filename string `json:"filename"`
        Data     string `json:"data"` // Base64
    }
    json.Unmarshal([]byte(req.Body), &uploadReq)

    // Base64デコード
    data, err := base64.StdEncoding.DecodeString(uploadReq.Data)
    if err != nil {
        return errorJSON(400, "invalid base64 data")
    }

    // S3にアップロード
    imagesBucket := os.Getenv("IMAGES_BUCKET")
    key := fmt.Sprintf("images/%s", uploadReq.Filename)
    _, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
        Bucket: &imagesBucket,
        Key:    &key,
        Body:   bytes.NewReader(data),
    })

    if err != nil {
        return errorJSON(500, "upload failed")
    }

    // 画像URLを返却
    imageURL := fmt.Sprintf("http://localhost:4566/%s/%s", imagesBucket, key)
    return jsonOK(map[string]string{"url": imageURL}), nil
}
```

### 7. 監視・ロギング

#### 構造化ログ

**目的**: JSON形式の構造化ログでデバッグを効率化

**実装例**:

```go
import (
    "log/slog"
    "os"
)

var logger *slog.Logger

func init() {
    logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

// ログ出力例
logger.Info("post created",
    slog.Int("post_id", post.ID),
    slog.String("title", post.Title),
    slog.String("user_id", userID),
)
```

#### メトリクス収集

**目的**: API使用状況の監視

**実装例**:

```go
// カスタムメトリクス（CloudWatch）
import (
    "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
    "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

func recordMetric(ctx context.Context, metricName string, value float64) {
    cwClient := cloudwatch.NewFromConfig(cfg)
    _, err := cwClient.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
        Namespace: aws.String("BlogAPI"),
        MetricData: []types.MetricDatum{
            {
                MetricName: aws.String(metricName),
                Value:      aws.Float64(value),
                Unit:       types.StandardUnitCount,
            },
        },
    })
    if err != nil {
        logger.Error("failed to record metric", slog.String("error", err.Error()))
    }
}

// 使用例
recordMetric(ctx, "PostCreated", 1)
recordMetric(ctx, "PostViewed", 1)
```

## 実装の優先順位

1. **レンダリング層** - ユーザー体験の向上
2. **キャッシュ** - 性能改善
3. **認証** - セキュリティ
4. **検索** - 機能拡張
5. **監視** - 運用改善
6. **データベース統合** - スケーラビリティ

## 注意事項

### LocalStack の制限

- **Cognito**: Pro版のみ対応
- **CloudFront**: Pro版のみ対応
- **一部のAPI Gateway機能**: 制限あり

### 性能考慮

- Lambda の Cold Start を考慮した実装
- S3 の読み取り頻度とコスト
- DynamoDB の読み書きキャパシティ

### セキュリティ

- 入力値の検証
- SQLインジェクション対策（DynamoDB使用時）
- CORS設定の適切な制限

これらの拡張により、基本的なブログAPIから実用的なWebアプリケーションへと発展させることができます。
