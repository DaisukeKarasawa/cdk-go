# 準備段階

## 概要

リファクタリングを開始する前に、現状を正確に把握し、改善計画を立てます。

**目的**: 現行実装の課題を特定し、リファクタリング方針を決定する

**期待される効果**: 効率的で安全なリファクタリングの実行

**リスク**: 現状把握不足による不適切な改善方針

## 現状分析

### Before（現在の実装）

#### Lambda関数の現状

**ファイル**: `lambda/cmd/blog/main.go`（134行）

**構造**:

```go
// 現在の構造（要約）
package main

import (
    // 多数のimport
)

type Post struct { ... }

var (
    s3Client *s3.Client
    bucket   string
)

func init() { ... }
func jsonOK(v interface{}) events.APIGatewayProxyResponse { ... }
func errorJSON(code int, msg string) (events.APIGatewayProxyResponse, error) { ... }
func handle(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    // 134行の巨大なハンドラー関数
    // - ルーティング
    // - ビジネスロジック
    // - S3アクセス
    // - エラーハンドリング
    // すべてが1つの関数に混在
}
func main() { ... }
```

**問題点**:

- **単一責任原則違反**: 1つの関数で複数の責務を担当
- **テスト困難**: 巨大な関数のテストが困難
- **再利用性なし**: ビジネスロジックがハンドラーに密結合
- **エラーハンドリング**: 統一されていないエラー処理
- **ログ不足**: デバッグに必要なログが不足

#### CDKスタックの現状

**ファイル**: `cdk-go.go`（88行）

**構造**:

```go
// 現在の構造（要約）
func NewCdkGoStack(scope constructs.Construct, id string, props *CdkGoStackProps) awscdk.Stack {
    // S3、Lambda、API Gatewayの定義が1つの関数に混在
    bucket := awss3.NewBucket(stack, jsii.String("BlogPosts"), &awss3.BucketProps{})
    fn := awslambda.NewFunction(stack, jsii.String("BlogApi"), &awslambda.FunctionProps{...})
    api := awsapigateway.NewLambdaRestApi(stack, jsii.String("BlogApiGateway"), &awsapigateway.LambdaRestApiProps{...})
    return stack
}
```

**問題点**:

- **モジュール化不足**: すべてのリソースが1つの関数に定義
- **設定のハードコーディング**: 環境別設定ができない
- **再利用性なし**: Constructとして独立していない
- **タグ付けなし**: リソース管理に必要なタグが不足

## リファクタリング手順

### 1. 現状の動作確認

**目的**: リファクタリング前のベースラインを確立

```bash
# LocalStackの動作確認
docker compose ps

# APIの動作確認
REGION=${AWS_DEFAULT_REGION:-ap-northeast-1}
REST_API_ID=$(awslocal --region "$REGION" apigateway get-rest-apis | jq -r '.items[0].id')
BASE="http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_"

echo "Testing current API..."
echo "1. GET /posts"
curl -s "${BASE}/posts" | jq .

echo "2. POST /posts"
curl -s -X POST \
  -H "Content-Type: application/json" \
  -d '{"id":1,"title":"Test","content":"Test content"}' \
  "${BASE}/posts" | jq .

echo "3. GET /posts/1"
curl -s "${BASE}/posts/1" | jq .
```

**期待結果**: すべてのAPIエンドポイントが正常に動作する

### 2. Git管理の確認

**目的**: 安全なリファクタリングのためのバージョン管理

```bash
# 現在の状態をコミット
git add .
git commit -m "Before refactoring: baseline state"

# リファクタリング用ブランチを作成
git checkout -b refactoring/phase-1-preparation

# ブランチの確認
git branch
```

### 3. コードメトリクスの収集

**目的**: 改善前の数値を記録

```bash
# Lambda関数の行数
wc -l lambda/cmd/blog/main.go

# CDKスタックの行数
wc -l cdk-go.go

# 依存関係の確認
go mod graph | wc -l
```

**記録例**:

```
lambda/cmd/blog/main.go: 134 lines
cdk-go.go: 88 lines
Dependencies: 15 packages
```

### 4. パフォーマンス測定

**目的**: 改善前の性能ベースラインを確立

```bash
# API応答時間の測定
time curl -s "${BASE}/posts" > /dev/null

# Lambda Cold Start時間の測定（初回実行）
time curl -s "${BASE}/posts" > /dev/null

# 複数回実行での平均時間
for i in {1..5}; do
  echo "Request $i:"
  time curl -s "${BASE}/posts" > /dev/null
done
```

### 5. 課題の特定と優先順位付け

**目的**: 改善すべき課題を明確化

#### 高優先度（Critical）

1. **コードの可読性**: 134行の巨大なハンドラー関数
2. **テストカバレッジ**: テストが存在しない
3. **エラーハンドリング**: 統一されていない

#### 中優先度（High）

1. **パフォーマンス**: Cold Start対策なし
2. **ログ**: デバッグ情報が不足
3. **設定管理**: ハードコーディングされた設定

#### 低優先度（Medium）

1. **モジュール化**: CDKスタックの構造化
2. **セキュリティ**: IAM権限の最小化
3. **監視**: メトリクス収集なし

### 6. リファクタリング計画の策定

**目的**: 段階的な改善計画の決定

#### Phase 1: コード品質改善

- ハンドラーとビジネスロジックの分離
- パッケージ構成の整理
- エラーハンドリングの統一

#### Phase 2: パフォーマンス最適化

- S3アクセスの最適化
- Cold Start対策
- キャッシング戦略

#### Phase 3: テスト追加

- ユニットテストの追加
- 統合テストの実装
- E2Eテストの追加

#### Phase 4: インフラ改善

- CDKスタックの構造化
- 環境別設定の分離
- セキュリティ強化

### 7. 改善目標の設定

**目的**: 測定可能な改善目標の設定

#### コード品質目標

- **関数の行数**: 134行 → 30行以下
- **パッケージ数**: 1個 → 5個以上
- **循環的複雑度**: 高 → 低

#### パフォーマンス目標

- **Cold Start時間**: 現在の測定値 → 50%削減
- **API応答時間**: 現在の測定値 → 30%削減
- **メモリ使用量**: 現在の測定値 → 20%削減

#### テスト目標

- **カバレッジ**: 0% → 80%以上
- **テスト数**: 0個 → 20個以上

## 動作確認

### 現状確認の完了チェック

- [ ] LocalStackが正常に動作している
- [ ] すべてのAPIエンドポイントが応答する
- [ ] Gitで現在の状態がコミットされている
- [ ] リファクタリング用ブランチが作成されている
- [ ] コードメトリクスが記録されている
- [ ] パフォーマンスベースラインが測定されている
- [ ] 改善課題が特定されている
- [ ] リファクタリング計画が策定されている

### 期待結果

準備段階が完了すると、以下が整備されます：

1. **安全な作業環境**: Git管理とバックアップ
2. **明確な改善目標**: 測定可能な目標設定
3. **段階的計画**: リスクを最小化した改善計画
4. **ベースライン**: 改善前の性能・品質指標

## トラブルシューティング

### APIが応答しない

**症状**: curlコマンドで404エラー

**原因**: LocalStackの再起動やAPI Gatewayの設定問題

**解決策**:

```bash
# LocalStackの再起動
docker compose down
docker compose up -d

# API Gatewayの再確認
awslocal apigateway get-rest-apis | jq
```

### Git管理の問題

**症状**: コミットできない、ブランチが作成できない

**原因**: ファイルの変更が未ステージング

**解決策**:

```bash
# 変更状況の確認
git status

# すべての変更をステージング
git add .

# コミット
git commit -m "Before refactoring: baseline state"
```

### パフォーマンス測定の失敗

**症状**: timeコマンドで正確な測定ができない

**原因**: ネットワーク遅延やLocalStackの負荷

**解決策**:

```bash
# 複数回実行して平均を取る
for i in {1..10}; do
  echo "Request $i:"
  time curl -s "${BASE}/posts" > /dev/null 2>&1
done
```

## 次のステップ

準備段階が完了したら、[コード品質改善](../refactoring/02-code-quality.md)に進んでください。

**準備完了の確認**:

- [ ] 現状分析が完了している
- [ ] 改善計画が策定されている
- [ ] ベースラインが測定されている
- [ ] Git管理が整備されている

---

> **💡 ヒント**: 準備段階は時間をかけて丁寧に行うことが重要です。現状を正確に把握することで、その後のリファクタリングが効率的に進められます。
