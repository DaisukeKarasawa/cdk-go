# LocalStack セットアップ

## 概要

LocalStack を Docker 上で起動し、S3 等の基本動作を確認します。

**目的**: LocalStackが正しく起動しAWS互換エンドポイントが機能していること、ダミー資格情報での操作が通ることを確認

**リスク**: ポート競合（デフォルト: 4566）

## Docker Compose 設定

プロジェクト直下に `docker-compose.yml` を作成します：

```yaml
# docker-compose.yml（プロジェクト直下に作成）
# LocalStack Community 版の最小構成
# CloudFront 等 Pro 専用は利用しません
services:
  localstack:
    image: localstack/localstack:latest
    container_name: localstack
    ports:
      - "4566:4566" # Edge port
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

## 起動と確認

### 1. LocalStack 起動

```bash
docker compose up -d
```

### 2. 稼働確認

```bash
docker compose ps
```

### 3. S3 の疎通確認

バケット作成→一覧で基本動作を確認：

```bash
awslocal s3 mb s3://blog-posts
awslocal s3 ls
```

期待結果：

```
2024-01-01 12:00:00 blog-posts
```

## 重要な設定項目

### SERVICES 環境変数

CDK の bootstrap では以下のサービスが必要です：

- `s3` - アセット格納用
- `lambda` - Lambda関数実行
- `apigateway` - REST API
- `cloudformation` - CDKスタック管理
- `iam` - 権限管理
- `logs` - CloudWatch Logs
- `ssm` - Parameter Store（bootstrap用）
- `sts` - Security Token Service（bootstrap用）
- `ecr` - Container Registry（bootstrap用）

### ポート設定

- `4566` - メインのエッジポート（すべてのサービス）
- `4571` - 追加ポート

## トラブルシューティング

### ポート競合

```bash
# ポート使用状況確認
lsof -i :4566

# 既存プロセス終了後に再起動
docker compose down
docker compose up -d
```

### サービス有効化確認

```bash
# LocalStack のサービス状況確認
curl http://localhost:4566/_localstack/health | jq
```

## 補足

- LocalStack 上のリソースはすべてローカルに閉じます（課金なし）
- Community版では一部のサービス（CloudFront等）は利用できません

## 次のステップ

LocalStackが正常に起動したら、[CDK プロジェクト](./03-cdk-project.md)の初期化に進んでください。
