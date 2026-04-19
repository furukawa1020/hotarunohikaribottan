# API リファレンス

蛍の光ボタン (Hotaru End Button) のREST APIドキュメントです。

## ベースURL

```
http://localhost:8080
```

## 認証

すべてのAPIエンドポイントはZoom Apps SDKが発行する `x-zoom-app-context` ヘッダーを使用して認証します。

開発・デバッグ時はクエリパラメータでルームIDとユーザーIDを指定することもできます：

| パラメータ | 説明 | デフォルト |
|-----------|------|-----------|
| `roomId` | ルーム（ミーティング）ID | `public-room` |
| `pid` | 参加者ID | `anonymous-user` |

---

## エンドポイント一覧

### GET /api/state

現在の投票状態を取得します。参加者として登録され、最新のゲージHTMLが返されます。

**リクエスト**

```
GET /api/state
```

**レスポンス**

`Content-Type: text/html; charset=utf-8`

ゲージのHTMLフラグメントが返されます（HTMX用）。

```html
<div id="gauge-container">
  <div class="gauge">
    <div class="gauge-fill" style="width: 40.0%;"></div>
  </div>
  <p class="status-text">そろそろ… <span class='anonym-info'>(匿名)</span></p>
</div>
```

**ステータスコード**

| コード | 説明 |
|--------|------|
| `200 OK` | 成功 |
| `405 Method Not Allowed` | GETメソッド以外を使用した場合 |
| `500 Internal Server Error` | サーバー内部エラー |

---

### POST /api/vote

投票を行います。同一ユーザーの重複投票は無視されます。

**リクエスト**

```
POST /api/vote
```

**レスポンス**

`Content-Type: text/html; charset=utf-8`

投票後の最新ゲージHTMLフラグメントが返されます（`/api/state` と同形式）。

過半数に達した場合は、蛍の光を再生するスクリプトタグが含まれます：

```html
<div id="gauge-container">
  ...
  <script>if(window.hotaruAudio && window.hotaruAudio.paused) window.hotaruAudio.play();</script>
</div>
```

**ステータスコード**

| コード | 説明 |
|--------|------|
| `200 OK` | 成功（重複投票の場合も200） |
| `405 Method Not Allowed` | POSTメソッド以外を使用した場合 |
| `500 Internal Server Error` | サーバー内部エラー |

---

### GET /docs/{filename}

指定したMarkdownドキュメントファイルを取得します。

**リクエスト**

```
GET /docs/{filename}
```

**パスパラメータ**

| パラメータ | 説明 | 例 |
|-----------|------|-----|
| `filename` | 取得するMarkdownファイル名（`.md`省略可） | `API`, `REQUIREMENTS`, `ZOOM_SETUP` |

**レスポンス**

`Content-Type: text/plain; charset=utf-8`

Markdownファイルの内容がそのまま返されます。

**ステータスコード**

| コード | 説明 |
|--------|------|
| `200 OK` | 成功 |
| `400 Bad Request` | ファイル名が指定されていない場合 |
| `403 Forbidden` | パストラバーサル等の不正なパスが指定された場合 |
| `404 Not Found` | ファイルが存在しない場合 |

**使用例**

```bash
# API.md を取得
curl http://localhost:8080/docs/API

# REQUIREMENTS.md を取得
curl http://localhost:8080/docs/REQUIREMENTS
```
