# 蛍の光ボタン (Hotaru End Button)

長引く会議を「空気」で終わらせるための匿名投票Zoom Appです。

## 概要

参加者の過半数が「帰る」ボタンを押すと、全員に蛍の光が流れて会議終了を促します。
投票は完全匿名で行われ、誰が押したかは分かりません。

## 機能

- **匿名投票**: 参加者は誰が投票したか分からない
- **リアルタイムゲージ**: 投票状況をゲージで表示
- **自動トリガー**: 過半数に達すると蛍の光が再生される
- **Zoom App対応**: Zoomのサイドパネルで動作

## セットアップ

### 必要環境

- Go 1.21+
- Redis（任意、未設定時はインメモリストアを使用）
- Zoom開発者アカウント（本番利用時）

### ローカル起動

```bash
cd backend
go run .
```

サーバーはポート `8080` で起動します。

### 環境変数

| 変数名 | 説明 | デフォルト |
|--------|------|-----------|
| `PORT` | サーバーポート番号 | `8080` |
| `REDIS_URL` | Redis接続URL | （未設定時はインメモリ） |
| `ZOOM_CLIENT_SECRET` | ZoomアプリのClient Secret | （開発用ダミー値） |

### Docker

```bash
docker build -t hotaruend .
docker run -p 8080:8080 hotaruend
```

## API

APIエンドポイントの詳細は [API.md](API.md) を参照してください。

## Zoomセットアップ

Zoom Appとしてのセットアップ手順は [ZOOM_SETUP.md](ZOOM_SETUP.md) を参照してください。

## 要件定義

詳細な要件は [REQUIREMENTS.md](REQUIREMENTS.md) を参照してください。

## ライセンス

MIT
