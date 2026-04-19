# 🪲 蛍の光ボタン (Hotaru End Button)

> "半数が押したら、帰らなきゃいけない本能を呼び起こす"

長引く会議を、**匿名の合意形成**によって自然に終わらせる Zoom App です。  
半数の参加者が「帰る」ボタンを押すと、全員の画面に「蛍の光」が流れ出し、帰宅への強制的な合図を届けます。

---

## 概要

「帰りたいけど言い出せない……」という心理的ハードルを、論理ではなく**文化的条件反射（蛍の光のメロディ）**で突破するアプリです。

- **完全匿名投票**: 誰が押したか、ログを含め一切追跡不可
- **半数到達で自動演出**: 視覚フェードと BGM（`hotaru-piano.mp3`）の同時再生
- **ホスト向け通知**: 閾値到達時にホストへ「会議を終了してください」の案内を表示
- **データ無保存の原則**: 個人情報は一切 DB に保存しない

---

## スクリーンショット

| 待機中 | 押下後 | 蛍の光モード（エンディング） |
|---|---|---|
| ゲージが空の初期画面 | ゲージが更新され、ボタンが無効化 | 画面暗転 + BGM 再生 |

---

## 技術スタック

| 層 | 技術 |
|---|---|
| バックエンド | Go (net/http), Redis |
| フロントエンド | HTML / CSS / JavaScript, [HTMX](https://htmx.org/) |
| リアルタイム通信 | HTMX ポーリング（`/api/state`） |
| Zoom 連携 | [Zoom Apps SDK](https://appssdk.zoom.us/sdk.min.js) |
| コンテナ | Docker (マルチステージビルド) |

---

## ディレクトリ構成

```
.
├── backend/
│   ├── main.go             # HTTP サーバー、エンドポイント定義
│   ├── auth.go             # Zoom Apps 認証ミドルウェア (JWT/HMAC)
│   ├── redis_store.go      # Redis を使った状態管理 (投票・Presence)
│   └── redis_store_test.go # ストアのユニットテスト
├── frontend/
│   ├── index.html          # メイン画面 (HTMX + Zoom SDK)
│   ├── style.css           # スタイルシート
│   ├── zoom-init.js        # Zoom Apps SDK の初期化と投票処理
│   └── hotaru-piano.mp3    # エンディング BGM
├── Dockerfile              # コンテナビルド定義
├── REQUIREMENTS.md         # 要件定義書（詳細仕様）
└── ZOOM_SETUP.md           # Zoom Marketplace 登録・ローカルテスト手順
```

---

## セットアップ

### 前提条件

- [Go](https://go.dev/) 1.21 以上
- [Redis](https://redis.io/) 7 以上（または Docker）
- [ngrok](https://ngrok.com/)（Zoom Apps の HTTPS 要件のため）
- Zoom アカウント（Developer アカウント推奨）

---

### 1. リポジトリのクローン

```bash
git clone https://github.com/furukawa1020/hotarunohikaribottan.git
cd hotarunohikaribottan
```

### 2. 環境変数の設定

バックエンド起動に必要な環境変数を設定します。

```bash
# Redis の接続先（デフォルト: localhost:6379）
export REDIS_ADDR=localhost:6379

# Zoom Apps の Client Secret（認証に使用）
export ZOOM_APP_CLIENT_SECRET=your_client_secret_here

# サーバーのポート番号（省略時: 8080）
export PORT=8080
```

### 3. Redis の起動

```bash
# Docker を使う場合
docker run -d -p 6379:6379 redis:7-alpine
```

### 4. バックエンドのビルドと起動

```bash
cd backend
go mod download
go run .
```

サーバーが `http://localhost:8080` で起動します。

### 5. Docker を使う場合（一括起動）

```bash
docker build -t hotaruend .
docker run -p 8080:8080 -e ZOOM_APP_CLIENT_SECRET=your_secret hotaruend
```

---

## Zoom App としての設定

実際の Zoom ミーティング内で動かすには、ngrok 経由で HTTPS URL を取得し、Zoom Marketplace にアプリを登録する必要があります。  
詳細な手順は **[ZOOM_SETUP.md](./ZOOM_SETUP.md)** を参照してください。

---

## API エンドポイント

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/state` | 現在の投票ゲージ HTML を返す（HTMX ターゲット用） |
| `POST` | `/api/vote` | 投票を記録し、更新後のゲージ HTML を返す |

すべてのエンドポイントは Zoom Apps の認証ヘッダー（`x-zoom-app-context`）による検証が必要です。

---

## 仕組み

```
参加者がアプリを開く
  → PRESENCE_JOIN: Redis に参加者として登録（母数+1）

「帰る」ボタンを押す
  → VOTE_CAST: HMAC ハッシュ化した参加者 ID で重複チェック後、投票数+1

投票数 >= ceil(母数 / 2) に達したとき
  → END_TRIGGERED: 全クライアントが「蛍の光モード」に遷移
  → hotaru-piano.mp3 の再生開始
  → ホストに「会議を終了してください」を表示

アプリを閉じる / タイムアウト
  → PRESENCE_LEAVE: 母数-1
```

**プライバシーについて**: 参加者 ID は HMAC ハッシュ化してのみ使用され、サーバーには一切平文で保存されません。会議終了後、全データは TTL（24 時間以内）で自動削除されます。

---

## テスト

```bash
cd backend
go test ./...
```

---

## 詳細仕様

詳しい要件・状態遷移・エッジケース対応については **[REQUIREMENTS.md](./REQUIREMENTS.md)** を参照してください。

---

## ライセンス

このプロジェクトは MIT ライセンスのもとで公開されています。
