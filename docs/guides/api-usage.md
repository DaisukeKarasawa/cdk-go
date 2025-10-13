# API 使用方法

## 概要

デプロイされたブログAPIのCRUD操作方法を説明します。

**前提条件**:

- LocalStackが起動している
- CDKスタックがデプロイ済み
- [CRUD対応Lambda](../reference/crud-lambda.md)が実装済み（最小実装の場合はGETのみ）

## API エンドポイント取得

### REST API ID の取得

```bash
# リージョンの決定（Stack ARNのリージョンに合わせる / 既定ap-northeast-1）
REGION=${AWS_DEFAULT_REGION:-ap-northeast-1}

# REST API ID の取得
REST_API_ID=$(awslocal --region "$REGION" apigateway get-rest-apis | jq -r '.items[0].id')
echo "REST API ID: $REST_API_ID"

# ベースURL
BASE="http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_"
echo "API Base URL: $BASE"
```

## CRUD 操作

### 1. 記事作成（POST /posts）

```bash
curl -s -X POST \
  -H "Content-Type: application/json" \
  -d '{"id":1,"title":"Hello","content":"# Hello from API\nThis is markdown content."}' \
  "${BASE}/posts" | jq .
```

**期待レスポンス**:

```json
{
  "id": 1,
  "title": "Hello",
  "content": "# Hello from API\nThis is markdown content."
}
```

### 2. 記事一覧取得（GET /posts）

```bash
curl -s "${BASE}/posts" | jq .
```

**期待レスポンス**:

```json
[
  {
    "id": 1,
    "title": "Hello",
    "content": "# Hello from API\nThis is markdown content."
  }
]
```

### 3. 記事個別取得（GET /posts/{id}）

```bash
curl -s "${BASE}/posts/1" | jq .
```

**期待レスポンス**:

```json
{
  "id": 1,
  "title": "Hello",
  "content": "# Hello from API\nThis is markdown content."
}
```

### 4. 記事更新（PUT /posts/{id}）

```bash
curl -s -X PUT \
  -H "Content-Type: application/json" \
  -d '{"title":"Hello (updated)","content":"# Updated\nNew content."}' \
  "${BASE}/posts/1" | jq .
```

**期待レスポンス**:

```json
{
  "id": 1,
  "title": "Hello (updated)",
  "content": "# Updated\nNew content."
}
```

### 5. 記事削除（DELETE /posts/{id}）

```bash
curl -s -X DELETE "${BASE}/posts/1" -i | head -n1
```

**期待レスポンス**:

```
HTTP/1.1 204 No Content
```

### 6. 削除確認（GET /posts/{id} → 404）

```bash
curl -s -o /dev/null -w "%{http_code}\n" "${BASE}/posts/1"
```

**期待レスポンス**:

```
404
```

## 一連のテストスクリプト

すべてのCRUD操作を順番に実行するスクリプト：

```bash
#!/bin/bash

# 設定
REGION=${AWS_DEFAULT_REGION:-ap-northeast-1}
REST_API_ID=$(awslocal --region "$REGION" apigateway get-rest-apis | jq -r '.items[0].id')
BASE="http://localhost:4566/restapis/${REST_API_ID}/prod/_user_request_"

echo "Testing API: $BASE"

# 1) 作成
echo "1. Creating post..."
curl -s -X POST \
  -H "Content-Type: application/json" \
  -d '{"id":1,"title":"Hello","content":"# Hello from API\nThis is markdown content."}' \
  "${BASE}/posts" | jq .

# 2) 一覧
echo "2. Listing posts..."
curl -s "${BASE}/posts" | jq .

# 3) 取得
echo "3. Getting post..."
curl -s "${BASE}/posts/1" | jq .

# 4) 更新
echo "4. Updating post..."
curl -s -X PUT \
  -H "Content-Type: application/json" \
  -d '{"title":"Hello (updated)","content":"# Updated\nNew content."}' \
  "${BASE}/posts/1" | jq .

# 5) 削除
echo "5. Deleting post..."
curl -s -X DELETE "${BASE}/posts/1" -i | head -n1

# 6) 削除確認
echo "6. Confirming deletion (should be 404)..."
curl -s -o /dev/null -w "%{http_code}\n" "${BASE}/posts/1"
```

## S3 直接操作（参考）

APIを使わずにS3に直接データを配置する場合：

### サンプルデータ作成

```bash
# サンプル記事（JSON）
cat > hello.json <<'EOF'
{"id":1,"title":"Hello","content":"# Hello from LocalStack\nThis is a sample."}
EOF
```

### S3 アップロード

```bash
# デプロイ済みバケット名の特定
POSTS_BUCKET=$(awslocal s3 ls | awk '{print $3}' | grep -i blogposts | head -n1)
echo "Bucket: $POSTS_BUCKET"

# S3 にアップロード（キーは posts/{id}.json）
awslocal s3 cp hello.json s3://$POSTS_BUCKET/posts/1.json

# 確認
awslocal s3 ls s3://$POSTS_BUCKET/posts/
```

## エラーハンドリング

### よくあるエラー

#### 400 Bad Request

```json
{ "error": "invalid body: require id,title,content" }
```

- **原因**: リクエストボディの形式が不正
- **解決**: JSON形式とフィールド（id, title, content）を確認

#### 404 Not Found

```json
{ "error": "not found" }
```

- **原因**: 指定されたIDの記事が存在しない
- **解決**: 存在するIDを指定するか、先に作成

#### 500 Internal Server Error

```json
{ "error": "create failed" }
```

- **原因**: S3への書き込みに失敗
- **解決**: Lambda関数のログを確認（[運用手順](./operations.md#ログ確認)参照）

## 最小実装での制限

[最小実装](../getting-started/04-lambda-development.md)の場合：

- **GET /posts**: 固定のモックデータを返却
- **GET /posts/{id}**: 固定のMarkdownテキストを返却
- **POST/PUT/DELETE**: 未実装（404エラー）

完全なCRUD機能を使用するには、[CRUD対応Lambda](../reference/crud-lambda.md)の実装に置き換えてください。

---

> **💡 Docker環境での開発**: 依存関係の更新が必要な場合は `docker compose exec go-dev go mod tidy` を実行してください。
