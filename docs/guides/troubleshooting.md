# トラブルシューティング

## CDK デプロイ関連の問題

### `cdklocal bootstrap` / `deploy` で失敗する

#### 1. LocalStack の起動確認

**症状**: bootstrap や deploy コマンドが失敗する

**確認方法**:

```bash
docker compose ps
```

**解決策**: LocalStack コンテナが起動していない場合

```bash
docker compose up -d
```

#### 2. 環境変数の設定確認

**確認方法**:

```bash
echo $AWS_DEFAULT_REGION
echo $AWS_ACCESS_KEY_ID
echo $AWS_SECRET_ACCESS_KEY
```

**解決策**: 環境変数を再設定

```bash
export AWS_DEFAULT_REGION=ap-northeast-1
export AWS_ACCESS_KEY_ID=dummy
export AWS_SECRET_ACCESS_KEY=dummy
```

#### 3. cdklocal が aws-cdk を見つけられない（MODULE_NOT_FOUND）

**症状**: `Error: Cannot find module 'aws-cdk'`

**原因**: cdklocal は内部で `aws-cdk` のAPIを呼ぶため、同一NODE_PATH/依存に `aws-cdk` が必要

**解決策（推奨）**: プロジェクトへローカル導入して npx 経由で実行

```bash
npm install -D aws-cdk aws-cdk-local
# 実行は npx cdklocal <cmd> / npx cdk <cmd>
```

**代替（グローバル）**:

```bash
npm install -g aws-cdk aws-cdk-local
export NODE_PATH=$(npm root -g)
```

**Homebrew 経由の aws-cdk を使用している場合**:

```bash
export NODE_PATH="$(brew --prefix aws-cdk)/libexec/lib/node_modules:$NODE_PATH"
```

#### 4. SSM が無効で bootstrap が失敗する

**症状**: `Service 'ssm' is not enabled. Please check your 'SERVICES' configuration variable.`

**解決策**: `docker-compose.yml` の `SERVICES` に `ssm, sts, ecr` を追加して LocalStack を再起動

```yaml
environment:
  - SERVICES=s3,lambda,apigateway,cloudformation,iam,logs,ssm,sts,ecr
```

```bash
docker compose down
docker compose up -d
```

#### 5. アセットが見つからない

**症状**: `panic: Cannot find asset at dist/blog.zip`

**原因**: CDK アプリ内で `awslambda.Code_FromAsset("dist/blog.zip")` を使用しており、ZIP が未作成

**解決策**: 事前に Lambda をビルドして ZIP を作成

```bash
mkdir -p dist/blog
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/blog/bootstrap lambda/cmd/blog
( cd dist/blog && zip -j ../blog.zip bootstrap )
```

### cdklocal のインストール問題

#### 1. PyPI からのインストール失敗

**症状**: `pipx install aws-cdk-local` でエラー（No matching distribution）

**原因**: `aws-cdk-local` はPyPIに存在しない

**解決策**: npm でインストール

```bash
npm install -g aws-cdk-local
# または
npm install -D aws-cdk-local  # プロジェクトローカル
```

#### 2. npx コマンドの誤用

**症状**: `npx install -g aws-cdk-local` でエラー（could not determine executable to run）

**原因**: `npx install -g` はコマンドとして無効

**解決策**:

```bash
npm install -g aws-cdk-local  # グローバル化
# または
npx cdklocal <cmd>  # 直接実行
```

#### 3. 既存 CDK との競合

**症状**: `npm install -g aws-cdk` で EEXIST（既存ファイルあり）

**確認**:

```bash
cdk --version  # 既にインストール済みかチェック
```

**解決策1**: 上書き（注意）

```bash
npm install -g aws-cdk --force
```

**解決策2**: 再インストール

```bash
npm uninstall -g aws-cdk && npm install -g aws-cdk
```

### CDK 初期化の問題

**症状**: `cdk init app --language go` が "cannot be run in a non-empty directory" で失敗

**原因**: CDKの初期化は空ディレクトリで行う必要がある

**解決策**: 新しいディレクトリを作成して移動

```bash
mkdir my-cdk-app && cd my-cdk-app && cdk init app --language go
```

## Go 環境関連の問題

### Go モジュール取得失敗

**症状**: unexpected EOF / proxyエラー等でモジュール取得に失敗

**原因**: ネットワークの一時的な断/タイムアウト、GOPROXY経由の取得失敗、バージョン解決の揺らぎ

**解決策**:

1. **リトライ**:

   ```bash
   go clean -modcache
   go mod tidy
   ```

2. **バージョン固定**:

   ```bash
   go get github.com/aws/aws-cdk-go/awscdk/v2@v2.219.0
   ```

3. **プロキシ切替**:

   ```bash
   export GOPROXY=https://proxy.golang.org,direct
   go mod tidy
   ```

4. **一時的設定調整**:
   ```bash
   export GOPRIVATE=""
   export GONOSUMDB=""
   go mod tidy
   ```

### awslambdagoalpha の取得問題

**症状**: `awslambdagoalpha` が取得できない / import解決できない

**原因**: v2配下以外のパス指定、または環境により `awslambdagoalpha` の取得が不安定

**解決策**: `awslambda.Code.fromAsset` でビルド済みZIPを配布する方式に切り替える（本手順の実装）

**補足**: ZIP方式はSDKや依存の揺らぎを避けやすく、CI上でも再現性が高い

### Go CDK v1/v2 の混在による型エラー

**症状**: `constructs/v3` と `constructs/v10`、`awscdk v1` と `awscdk/v2` の混在でビルドエラー

例: "does not implement constructs.Construct"

**解決策**:

- `go.mod` を `awscdk/v2` と `constructs/v10` に統一し、`constructs/v3` の間接参照を排除
- import を v2 配下に統一（例: `github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway`）
- 文字列は `jsii.String()` を使用、`GrantReadWrite` のシグネチャに合わせる（`bucket.GrantReadWrite(fn, nil)` など）

### Lambda アーキテクチャ不一致

**症状**: 実行時にアーキテクチャ不一致でエラー（例: x86_64想定のランタイムに arm64 バイナリを配置）

**解決策**:

- 既定では Lambda のアーキテクチャは x86_64。`GOOS=linux GOARCH=amd64` でビルド
- もし `arm64` を使う場合は、CDK 側で `Architecture_ARM_64` を指定する

```go
fn := awslambda.NewFunction(stack, awsString("BlogApi"), &awslambda.FunctionProps{
    Runtime:      awslambda.Runtime_PROVIDED_AL2(),
    Handler:      awsString("bootstrap"),
    Code:         awslambda.Code_FromAsset(awsString("dist/blog.zip"), nil),
    Architecture: awslambda.Architecture_ARM_64(), // ARM64 を使用する場合
    // ...
})
```

## LocalStack 関連の問題

### ポート競合

**症状**: LocalStack が起動しない、ポート 4566 が使用中

**確認**:

```bash
lsof -i :4566
```

**解決策**:

```bash
# 競合プロセスを終了
kill -9 <PID>

# または LocalStack を再起動
docker compose down
docker compose up -d
```

### サービスが有効化されていない

**症状**: 特定のAWSサービスが利用できない

**確認**:

```bash
curl http://localhost:4566/_localstack/health | jq
```

**解決策**: `docker-compose.yml` の `SERVICES` に必要なサービスを追加

```yaml
environment:
  - SERVICES=s3,lambda,apigateway,cloudformation,iam,logs,ssm,sts,ecr
```

### Docker ソケット権限エラー

**症状**: Lambda 実行時に Docker 関連のエラー

**解決策**: Docker Desktop が起動していることを確認

```bash
docker ps  # Docker が動作しているか確認
```

## API 関連の問題

### API が 404 を返す

**症状**: デプロイ後にAPI呼び出しで 404 エラー

**確認**:

```bash
# REST API の存在確認
awslocal apigateway get-rest-apis | jq

# Lambda 関数の存在確認
awslocal lambda list-functions | jq
```

**解決策**:

1. 正しい REST API ID を使用しているか確認
2. Lambda 関数が正常にデプロイされているか確認
3. API Gateway と Lambda の統合設定を確認

### Lambda 関数がタイムアウト

**症状**: API 呼び出しが長時間応答しない

**確認**:

```bash
awslocal logs tail "/aws/lambda/BlogApi" --follow
```

**解決策**:

1. Lambda 関数のタイムアウト設定を確認・延長
2. S3 への接続設定を確認
3. Lambda 関数内のエラーハンドリングを改善

### S3 権限エラー

**症状**: Lambda から S3 への読み書きでエラー

**確認**:

```bash
# S3 バケットの存在確認
awslocal s3 ls

# Lambda 関数の環境変数確認
awslocal lambda get-function --function-name BlogApi | jq '.Configuration.Environment'
```

**解決策**:

1. CDK で `bucket.GrantReadWrite(fn, nil)` が設定されているか確認
2. 環境変数 `POSTS_BUCKET` が正しく設定されているか確認
3. Lambda 関数を再デプロイ

## 一般的な対処手順

### 1. 基本確認

```bash
# LocalStack 状況
docker compose ps
curl http://localhost:4566/_localstack/health | jq

# 環境変数
echo $AWS_DEFAULT_REGION
echo $AWS_ACCESS_KEY_ID

# CDK 状況
npx cdklocal --version
```

### 2. ログ確認

```bash
# Lambda ログ
awslocal logs tail "/aws/lambda/BlogApi" --follow

# Docker ログ
docker compose logs localstack
```

### 3. リセット手順

```bash
# 完全リセット
cdklocal destroy --force
docker compose down
docker compose up -d

# 再構築
cdklocal bootstrap aws://000000000000/ap-northeast-1
make deploy  # または手動でビルド→デプロイ
```

問題が解決しない場合は、上記の手順を順番に実行してください。
