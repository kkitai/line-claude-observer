# line-claude-observer

LINEトークルームにボットを常駐させ、`/opinion` コマンドで直近の会話履歴を Claude に渡し、第三者目線の意見を返信するシステム。

## Application Overview

### 機能概要

- LINEグループ/ルームに招待するだけで全メッセージを自動蓄積
- `/opinion` コマンドを発言すると、直近 N 件の会話を Claude が分析
- 第三者目線のコメントをトークルームに返信

### アーキテクチャ

```
LINEトークルーム
    ↓ Webhook (全メッセージをリアルタイム受信)
Webhook サーバー (Go / chi)
    ↓ メッセージを PostgreSQL に蓄積
    ↓ /opinion を検知したら直近 N 件を取得
Claude API (claude-sonnet-4-6)
    ↓ 第三者目線で分析・意見生成
LINEトークルームへ返信
```

### 必要な外部サービス

| サービス | 用途 |
|----------|------|
| LINE Messaging API | ボットアカウント、Webhook受信、返信送信 |
| Anthropic Claude API | 第三者意見の生成 |
| PostgreSQL | メッセージ蓄積 |

## How to Run the Application

### 前提条件

- Go 1.23+
- Docker & Docker Compose
- LINE Messaging API チャネル（Channel Secret / Access Token）
- Anthropic API キー

### セットアップ

```bash
# 1. 依存パッケージのインストール
go mod download

# 2. 環境変数の設定
cp .env.example .env
# .env を編集して各種キーを設定

# 3. データベースの起動
docker compose up -d db

# 4. マイグレーションの実行
go run ./app/cmd/migrate/main.go up

# 5. サーバーの起動
go run ./app/cmd/server/main.go
```

### 環境変数

| 変数名 | 説明 | 必須 |
|--------|------|------|
| `LINE_CHANNEL_SECRET` | LINE チャネルシークレット | ✓ |
| `LINE_CHANNEL_ACCESS_TOKEN` | LINE チャネルアクセストークン | ✓ |
| `ANTHROPIC_API_KEY` | Anthropic API キー | ✓ |
| `DATABASE_URL` | PostgreSQL 接続文字列 | ✓ |
| `PORT` | サーバーポート（デフォルト: 8080） | |
| `CONTEXT_MESSAGE_COUNT` | Claude に渡す直近メッセージ数（デフォルト: 20） | |
| `OPINION_COMMAND` | トリガーコマンド（デフォルト: /opinion） | |

### LINE Webhook 設定

1. [LINE Developers Console](https://developers.line.biz/) でチャネルを作成
2. Webhook URL を `https://{your-domain}/webhook` に設定
3. ボットをLINEグループに招待

## How to Run Unit Tests

```bash
# 全ユニットテストの実行
go test ./app/internal/... -v

# カバレッジレポート付き
go test ./app/internal/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## How to Run E2E Tests

```bash
# 前提: テスト用DBが起動していること
docker compose up -d db

# E2Eテストの実行
go test ./app/e2e/... -v -tags=e2e
```
