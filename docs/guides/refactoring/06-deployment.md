# デプロイとバリデーション

## 概要

リファクタリング完了後のアプリケーションを本格的にデプロイし、動作確認とパフォーマンス検証を行います。

**目的**: リファクタリング後のデプロイ、動作確認、パフォーマンス比較、ロールバック手順の確立

**期待される効果**: 本格的なプロダクション環境への移行準備、品質保証の確立

**リスク**: デプロイ失敗、パフォーマンス劣化、データ損失

## 現状分析

### Before（リファクタリング前の状態）

**デプロイ状況**:

- **単純なデプロイ**: `cdklocal deploy`のみ
- **動作確認**: 手動でのAPI呼び出し
- **パフォーマンス測定**: なし
- **ロールバック**: 手動でのスタック破棄
- **監視**: 基本的なログのみ

**課題**:

- デプロイプロセスの標準化不足
- 自動化されたテストの不足
- パフォーマンス比較の不足
- ロールバック手順の不明確さ

## リファクタリング手順

### 1. デプロイスクリプトの作成

**目的**: デプロイプロセスの自動化

**ファイル**: `scripts/deploy.sh`

```bash
#!/bin/bash

set -e

# 設定
ENVIRONMENT=${1:-dev}
REGION=${2:-ap-northeast-1}
ACCOUNT=${3:-000000000000}

echo "🚀 Deploying Blog API to $ENVIRONMENT environment"

# 環境変数の設定
export CDK_ENVIRONMENT=$ENVIRONMENT
export CDK_DEFAULT_REGION=$REGION
export CDK_DEFAULT_ACCOUNT=$ACCOUNT
export AWS_DEFAULT_REGION=$REGION
export AWS_ACCESS_KEY_ID=dummy
export AWS_SECRET_ACCESS_KEY=dummy

# 事前チェック
echo "📋 Pre-deployment checks..."

# LocalStackの起動確認
if ! docker compose ps | grep -q "localstack.*Up"; then
    echo "❌ LocalStack is not running. Starting..."
    docker compose up -d localstack
    sleep 10
fi

# Lambda ZIPの存在確認
if [ ! -f "dist/blog.zip" ]; then
    echo "❌ Lambda ZIP not found. Building..."
    make build-docker
fi

# テストの実行
echo "🧪 Running tests..."
make test-docker

# スタックの合成
echo "🔨 Synthesizing stack..."
cdklocal synth

# デプロイ
echo "🚀 Deploying stack..."
cdklocal deploy --require-approval never

# デプロイ後の確認
echo "✅ Deployment completed successfully!"

# 出力情報の取得
echo "📊 Getting deployment outputs..."
API_ENDPOINT=$(awslocal apigateway get-rest-apis | jq -r '.items[0].id')
BUCKET_NAME=$(awslocal s3 ls | awk '{print $3}' | grep -i blogposts | head -n1)

echo "🌐 API Endpoint: http://localhost:4566/restapis/${API_ENDPOINT}/prod/_user_request_"
echo "📦 S3 Bucket: $BUCKET_NAME"

# 動作確認
echo "🔍 Running post-deployment validation..."
./scripts/validate.sh

echo "🎉 Deployment and validation completed successfully!"
```

### 2. バリデーションスクリプトの作成

**目的**: デプロイ後の動作確認

**ファイル**: `scripts/validate.sh`

```bash
#!/bin/bash

set -e

echo "🔍 Validating Blog API deployment"

# APIエンドポイントの取得
REGION=${AWS_DEFAULT_REGION:-ap-northeast-1}
REST_API_ID=$(awslocal --region "$REGION" apigateway get-rest-apis | jq -r '.items[0].id')
BASE="http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_"

echo "🌐 Testing API: $BASE"

# テスト結果の記録
TEST_RESULTS="validation-results.json"
echo "[]" > $TEST_RESULTS

# テスト関数
run_test() {
    local test_name="$1"
    local method="$2"
    local path="$3"
    local data="$4"
    local expected_status="$5"

    echo "🧪 Running test: $test_name"

    if [ "$method" = "GET" ]; then
        response=$(curl -s -w "%{http_code}" -o /tmp/response.json "${BASE}${path}")
    else
        response=$(curl -s -w "%{http_code}" -o /tmp/response.json \
            -X "$method" \
            -H "Content-Type: application/json" \
            -d "$data" \
            "${BASE}${path}")
    fi

    status_code="${response: -3}"
    response_body=$(cat /tmp/response.json)

    # 結果の記録
    jq --arg name "$test_name" \
       --arg method "$method" \
       --arg path "$path" \
       --arg status "$status_code" \
       --arg expected "$expected_status" \
       --arg body "$response_body" \
       '. += [{
           "test_name": $name,
           "method": $method,
           "path": $path,
           "status_code": $status,
           "expected_status": $expected,
           "response_body": $body,
           "passed": ($status == $expected)
       }]' $TEST_RESULTS > /tmp/validation.json && mv /tmp/validation.json $TEST_RESULTS

    if [ "$status_code" = "$expected_status" ]; then
        echo "✅ $test_name: PASSED ($status_code)"
    else
        echo "❌ $test_name: FAILED (expected $expected_status, got $status_code)"
        echo "   Response: $response_body"
    fi
}

# 1. 記事一覧取得（空の状態）
run_test "List Posts (Empty)" "GET" "/posts" "" "200"

# 2. 記事作成
run_test "Create Post" "POST" "/posts" '{"title":"Validation Test","content":"This is a validation test post."}' "201"

# 3. 記事一覧取得（データあり）
run_test "List Posts (With Data)" "GET" "/posts" "" "200"

# 4. 記事取得
run_test "Get Post" "GET" "/posts/1" "" "200"

# 5. 記事更新
run_test "Update Post" "PUT" "/posts/1" '{"title":"Updated Validation Test","content":"This is an updated validation test post."}' "200"

# 6. 記事削除
run_test "Delete Post" "DELETE" "/posts/1" "" "204"

# 7. 削除確認
run_test "Get Deleted Post" "GET" "/posts/1" "" "404"

# 8. 無効なリクエスト
run_test "Invalid Request" "POST" "/posts" '{"title":"","content":"Invalid"}' "400"

# 9. 存在しないリソース
run_test "Not Found" "GET" "/posts/999" "" "404"

# 結果の集計
echo "📊 Validation Results:"
passed=$(jq '[.[] | select(.passed == true)] | length' $TEST_RESULTS)
total=$(jq '. | length' $TEST_RESULTS)
failed=$((total - passed))

echo "✅ Passed: $passed"
echo "❌ Failed: $failed"
echo "📈 Success Rate: $((passed * 100 / total))%"

if [ $failed -gt 0 ]; then
    echo "❌ Validation failed. Check $TEST_RESULTS for details."
    exit 1
else
    echo "🎉 All validation tests passed!"
fi
```

### 3. パフォーマンス測定スクリプト

**目的**: リファクタリング前後の性能比較

**ファイル**: `scripts/benchmark.sh`

```bash
#!/bin/bash

set -e

echo "⚡ Running Blog API Performance Benchmark"

# APIエンドポイントの取得
REGION=${AWS_DEFAULT_REGION:-ap-northeast-1}
REST_API_ID=$(awslocal --region "$REGION" apigateway get-rest-apis | jq -r '.items[0].id')
BASE="http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_"

# ベンチマーク結果の記録
BENCHMARK_RESULTS="benchmark-results.json"
echo "{}" > $BENCHMARK_RESULTS

# パフォーマンス測定関数
measure_performance() {
    local operation="$1"
    local method="$2"
    local path="$3"
    local data="$4"
    local iterations="$5"

    echo "⚡ Measuring $operation performance ($iterations iterations)..."

    # ウォームアップ
    for i in {1..3}; do
        if [ "$method" = "GET" ]; then
            curl -s "${BASE}${path}" > /dev/null
        else
            curl -s -X "$method" \
                -H "Content-Type: application/json" \
                -d "$data" \
                "${BASE}${path}" > /dev/null
        fi
    done

    # 実際の測定
    times=()
    for i in $(seq 1 $iterations); do
        start_time=$(date +%s%N)

        if [ "$method" = "GET" ]; then
            curl -s "${BASE}${path}" > /dev/null
        else
            curl -s -X "$method" \
                -H "Content-Type: application/json" \
                -d "$data" \
                "${BASE}${path}" > /dev/null
        fi

        end_time=$(date +%s%N)
        duration=$((end_time - start_time))
        times+=($duration)
    done

    # 統計の計算
    total=0
    for time in "${times[@]}"; do
        total=$((total + time))
    done

    average=$((total / iterations))
    average_ms=$((average / 1000000))

    # 最小・最大の計算
    min=${times[0]}
    max=${times[0]}
    for time in "${times[@]}"; do
        if [ $time -lt $min ]; then min=$time; fi
        if [ $time -gt $max ]; then max=$time; fi
    done

    min_ms=$((min / 1000000))
    max_ms=$((max / 1000000))

    # 結果の記録
    jq --arg op "$operation" \
       --arg avg "$average_ms" \
       --arg min "$min_ms" \
       --arg max "$max_ms" \
       --arg iter "$iterations" \
       '.[$op] = {
           "average_ms": ($avg | tonumber),
           "min_ms": ($min | tonumber),
           "max_ms": ($max | tonumber),
           "iterations": ($iter | tonumber)
       }' $BENCHMARK_RESULTS > /tmp/benchmark.json && mv /tmp/benchmark.json $BENCHMARK_RESULTS

    echo "📊 $operation: Avg=${average_ms}ms, Min=${min_ms}ms, Max=${max_ms}ms"
}

# テストデータの準備
echo "📝 Preparing test data..."
curl -s -X POST \
    -H "Content-Type: application/json" \
    -d '{"title":"Benchmark Post 1","content":"Content for benchmark test 1"}' \
    "${BASE}/posts" > /dev/null

curl -s -X POST \
    -H "Content-Type: application/json" \
    -d '{"title":"Benchmark Post 2","content":"Content for benchmark test 2"}' \
    "${BASE}/posts" > /dev/null

# パフォーマンス測定
measure_performance "List Posts" "GET" "/posts" "" 10
measure_performance "Get Post" "GET" "/posts/1" "" 10
measure_performance "Create Post" "POST" "/posts" '{"title":"New Post","content":"New content"}' 5
measure_performance "Update Post" "PUT" "/posts/1" '{"title":"Updated Post","content":"Updated content"}' 5

# Cold Start測定
echo "❄️ Measuring Cold Start performance..."
# Lambda関数をクールダウンさせるため少し待機
sleep 30

cold_start_times=()
for i in {1..5}; do
    start_time=$(date +%s%N)
    curl -s "${BASE}/posts" > /dev/null
    end_time=$(date +%s%N)
    duration=$((end_time - start_time))
    cold_start_times+=($duration)
    sleep 30  # クールダウン
done

cold_start_total=0
for time in "${cold_start_times[@]}"; do
    cold_start_total=$((cold_start_total + time))
done

cold_start_avg=$((cold_start_total / 5))
cold_start_avg_ms=$((cold_start_avg / 1000000))

jq --arg avg "$cold_start_avg_ms" \
   '.cold_start_avg_ms = ($avg | tonumber)' $BENCHMARK_RESULTS > /tmp/benchmark.json && mv /tmp/benchmark.json $BENCHMARK_RESULTS

echo "❄️ Cold Start Average: ${cold_start_avg_ms}ms"

# 結果の表示
echo "📊 Performance Benchmark Results:"
cat $BENCHMARK_RESULTS | jq '.'

# クリーンアップ
echo "🧹 Cleaning up test data..."
curl -s -X DELETE "${BASE}/posts/1" > /dev/null
curl -s -X DELETE "${BASE}/posts/2" > /dev/null

echo "✅ Performance benchmark completed!"
```

### 4. ロールバックスクリプトの作成

**目的**: 問題発生時の迅速な復旧

**ファイル**: `scripts/rollback.sh`

```bash
#!/bin/bash

set -e

echo "🔄 Rolling back Blog API deployment"

# 設定
ENVIRONMENT=${1:-dev}
REGION=${2:-ap-northeast-1}
ACCOUNT=${3:-000000000000}

# 環境変数の設定
export CDK_ENVIRONMENT=$ENVIRONMENT
export CDK_DEFAULT_REGION=$REGION
export CDK_DEFAULT_ACCOUNT=$ACCOUNT
export AWS_DEFAULT_REGION=$REGION
export AWS_ACCESS_KEY_ID=dummy
export AWS_SECRET_ACCESS_KEY=dummy

# 確認
echo "⚠️  This will destroy all resources in the $ENVIRONMENT environment."
read -p "Are you sure you want to continue? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "❌ Rollback cancelled."
    exit 1
fi

# バックアップの作成
echo "💾 Creating backup..."
BACKUP_DIR="backup-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"

# S3データのバックアップ
BUCKET_NAME=$(awslocal s3 ls | awk '{print $3}' | grep -i blogposts | head -n1)
if [ -n "$BUCKET_NAME" ]; then
    echo "📦 Backing up S3 data..."
    awslocal s3 sync "s3://$BUCKET_NAME" "$BACKUP_DIR/s3-backup/" || true
fi

# Lambda関数のバックアップ
echo "📦 Backing up Lambda function..."
if [ -f "dist/blog.zip" ]; then
    cp "dist/blog.zip" "$BACKUP_DIR/lambda-backup.zip"
fi

# 設定ファイルのバックアップ
echo "📦 Backing up configuration..."
cp -r config/ "$BACKUP_DIR/" || true
cp cdk-go.go "$BACKUP_DIR/" || true

echo "✅ Backup created in $BACKUP_DIR"

# スタックの破棄
echo "🗑️  Destroying stack..."
cdklocal destroy --force

# 確認
echo "🔍 Verifying rollback..."
if ! awslocal apigateway get-rest-apis | jq -e '.items | length > 0' > /dev/null; then
    echo "✅ API Gateway destroyed"
else
    echo "⚠️  API Gateway still exists"
fi

if ! awslocal lambda list-functions | jq -e '.Functions | length > 0' > /dev/null; then
    echo "✅ Lambda functions destroyed"
else
    echo "⚠️  Lambda functions still exist"
fi

if ! awslocal s3 ls | grep -q blogposts; then
    echo "✅ S3 buckets destroyed"
else
    echo "⚠️  S3 buckets still exist"
fi

echo "🎉 Rollback completed successfully!"
echo "📁 Backup available in: $BACKUP_DIR"
```

### 5. CI/CDパイプラインの準備

**目的**: 継続的なデプロイの自動化

**ファイル**: `.github/workflows/deploy.yml`

```yaml
name: Deploy Blog API

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v3
        with:
          go-version: "1.23"

      - name: Set up Node.js
        uses: actions/setup-node@v3
        with:
          node-version: "20"

      - name: Install dependencies
        run: |
          npm install -g aws-cdk aws-cdk-local
          go mod download

      - name: Run tests
        run: make test-docker

      - name: Run validation
        run: ./scripts/validate.sh

  deploy-dev:
    needs: test
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v3

      - name: Set up Docker
        run: |
          docker compose up -d localstack
          sleep 10

      - name: Deploy to dev
        run: ./scripts/deploy.sh dev

      - name: Run benchmark
        run: ./scripts/benchmark.sh
```

### 6. 監視ダッシュボードの設定

**目的**: リアルタイム監視

**ファイル**: `scripts/setup-monitoring.sh`

```bash
#!/bin/bash

set -e

echo "📊 Setting up monitoring dashboard"

# APIエンドポイントの取得
REGION=${AWS_DEFAULT_REGION:-ap-northeast-1}
REST_API_ID=$(awslocal --region "$REGION" apigateway get-rest-apis | jq -r '.items[0].id')
BASE="http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_"

# ヘルスチェックエンドポイントの作成
echo "🏥 Setting up health check endpoint..."

# ヘルスチェック用のLambda関数を作成
cat > health-check.js << 'EOF'
exports.handler = async (event) => {
    return {
        statusCode: 200,
        body: JSON.stringify({
            status: 'healthy',
            timestamp: new Date().toISOString(),
            version: '1.0.0'
        })
    };
};
EOF

# 監視スクリプトの作成
cat > monitor.sh << 'EOF'
#!/bin/bash

while true; do
    echo "$(date): Checking API health..."

    # ヘルスチェック
    response=$(curl -s -w "%{http_code}" -o /dev/null "$BASE/posts")
    if [ "$response" = "200" ]; then
        echo "✅ API is healthy"
    else
        echo "❌ API is unhealthy (status: $response)"
    fi

    # パフォーマンス測定
    start_time=$(date +%s%N)
    curl -s "$BASE/posts" > /dev/null
    end_time=$(date +%s%N)
    duration=$((end_time - start_time))
    duration_ms=$((duration / 1000000))

    echo "📊 Response time: ${duration_ms}ms"

    sleep 60
done
EOF

chmod +x monitor.sh

echo "✅ Monitoring setup completed!"
echo "🚀 Run './monitor.sh' to start monitoring"
```

## 動作確認

### デプロイの実行

```bash
# 開発環境へのデプロイ
./scripts/deploy.sh dev

# 本番環境へのデプロイ
./scripts/deploy.sh prod ap-northeast-1 123456789012
```

### バリデーションの実行

```bash
# デプロイ後の動作確認
./scripts/validate.sh

# 結果の確認
cat validation-results.json | jq '.'
```

### パフォーマンス測定

```bash
# ベンチマークの実行
./scripts/benchmark.sh

# 結果の確認
cat benchmark-results.json | jq '.'
```

### 期待結果

- **デプロイ成功率**: 100%
- **バリデーション成功率**: 100%
- **API応答時間**: 200ms以下
- **Cold Start時間**: 1.5秒以下
- **エラー率**: 0%

## トラブルシューティング

### デプロイ失敗

**症状**: `cdklocal deploy`でエラー

**原因**: リソースの競合や権限不足

**解決策**:

```bash
# ロールバックの実行
./scripts/rollback.sh

# 問題の特定
cdklocal synth
awslocal logs describe-log-groups
```

### バリデーション失敗

**症状**: テストが失敗する

**原因**: APIの動作不良

**解決策**:

```bash
# 詳細なログ確認
awslocal logs tail "/aws/lambda/BlogApi" --follow

# 手動でのAPI確認
curl -v "http://localhost:4566/restapis/$(awslocal apigateway get-rest-apis | jq -r '.items[0].id')/prod/_user_request_/posts"
```

### パフォーマンス劣化

**症状**: 応答時間が期待値を超える

**原因**: リソース不足や設定問題

**解決策**:

```bash
# リソース使用量の確認
awslocal lambda get-function --function-name BlogApi | jq '.Configuration'

# メモリ設定の調整
# CDKスタックでMemorySizeを増加
```

## 次のステップ

デプロイとバリデーションが完了したら、リファクタリングフェーズは完了です。

**完了確認**:

- [ ] デプロイスクリプトが動作している
- [ ] バリデーションが100%成功している
- [ ] パフォーマンスが改善されている
- [ ] ロールバック手順が確立されている
- [ ] 監視が設定されている

## リファクタリング完了

🎉 **おめでとうございます！** リファクタリングフェーズが完了しました。

### 達成した改善

1. **コード品質**: 134行の単一ファイル → 構造化された複数パッケージ
2. **パフォーマンス**: Cold Start 2.5秒 → 1.5秒以下
3. **テストカバレッジ**: 0% → 80%以上
4. **インフラ**: 単一スタック → モジュール化されたConstruct
5. **運用**: 手動デプロイ → 自動化されたCI/CD

### 次のステップ

- [拡張ガイド](../reference/extensions.md)でさらなる機能追加
- [運用手順](./operations.md)で継続的な運用
- [トラブルシューティング](./troubleshooting.md)で問題解決

---

> **💡 ヒント**: リファクタリングは継続的なプロセスです。定期的にコードレビューとパフォーマンス測定を行い、継続的な改善を心がけてください。
