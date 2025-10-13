# 環境準備

## 概要

LocalStack + CDK（Go）でローカル完結の IaC/サーバーレス実行環境を整えます。

**目的**: クラウドに接続せずに、ローカルだけでAWS互換のAPIを呼び出し、CDKアプリの合成・デプロイ・検証が可能にする

**リスク**: バージョン不整合や PATH の競合

## 必要なツール

- Docker & Docker Compose
- Go (1.21+)
- Node.js (LTS)
- AWS CLI v2
- awslocal / cdklocal
- jq（レスポンス整形用）

## インストール手順

### 1. Homebrew 更新

```bash
brew update
```

### 2. Docker Desktop

https://www.docker.com/products/docker-desktop/ からインストール

インストール後に Docker Desktop を起動しておく

### 3. Go（1.21+ 推奨）

```bash
brew install go
go version  # 例: go version go1.22.x darwin/arm64 or amd64
```

### 4. Node.js（CDK CLI 用 / LTS推奨）

```bash
brew install node
node -v  # 例: v20.x
npm -v   # 例: 10.x
```

### 5. AWS CLI v2（任意。awslocal だけでもよい）

```bash
brew install awscli
aws --version
```

### 6. Python ツール（pipx経由で LocalStack ラッパー導入推奨）

```bash
brew install pipx
pipx ensurepath
```

### 7. awslocal / cdklocal の導入

```bash
# awslocal
pipx install awscli-local

# CDK関連（推奨: プロジェクトにローカル導入）
npm init -y >/dev/null 2>&1 || true
npm install -D aws-cdk aws-cdk-local
# その場実行: npx cdklocal <cmd> / npx cdk <cmd>
```

**代替（グローバル導入）**:

```bash
npm install -g aws-cdk aws-cdk-local
export NODE_PATH=$(npm root -g)  # cdklocal が aws-cdk を解決できない場合に必要
```

**Homebrew の aws-cdk を使う場合**:

```bash
export NODE_PATH="$(brew --prefix aws-cdk)/libexec/lib/node_modules:$NODE_PATH"
```

**動作確認**:

```bash
npx cdklocal --version
npx cdk --version  # ローカル導入時
# または
cdklocal --version # グローバル導入時
```

### 8. jq（レスポンス整形用。任意）

```bash
brew install jq
```

### 9. 環境変数（ローカル用ダミー資格情報）

LocalStack は任意の資格情報で可。固定しておくと便利。

```bash
export AWS_ACCESS_KEY_ID=dummy
export AWS_SECRET_ACCESS_KEY=dummy
export AWS_DEFAULT_REGION=ap-northeast-1
```

**プロファイルを作る場合（任意）**:

```bash
aws configure --profile localstack <<EOF
dummy
dummy
ap-northeast-1
json
EOF
```

## 補足

- LocalStack 用のラッパー `awslocal`/`cdklocal` を使うと `--endpoint-url` の指定が不要になり、設定漏れが減ります
- 注意: `npx install -g aws-cdk-local` は無効。グローバル化は `npm install -g` を使用

## 次のステップ

環境準備が完了したら、[LocalStack セットアップ](./02-localstack-setup.md)に進んでください。
