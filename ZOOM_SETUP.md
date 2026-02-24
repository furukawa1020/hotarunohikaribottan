# 蛍の光ボタン - Zoom App ローカルテスト手順

このWebアプリを実際にZoom内で「Zoom App」として動かすための手順です。
（※Zoomのサイドパネル内で `localhost` を直接開くことはできないため、`ngrok` 等でHTTPS化します）

## 1. アプリの公開 (ngrok の利用)
現在、バックエンドとフロントエンドを兼ねたGoサーバーが `http://localhost:8080` で動いています。
お手元のPCのターミナルで、ngrokを使ってこれを外部に公開します。

1. [ngrok](https://ngrok.com/) をインストールしていない場合はダウンロードします。
2. 以下のコマンドを実行してポート8080を公開します。
   ```bash
   ngrok http 8080
   ```
3. 表示される `Forwarding` のURL (例: `https://xxxx-xxx-xx-xx.ngrok-free.app`) をコピーします。これがアプリの本番URLになります。

## 2. Zoom Marketplace でのアプリ登録
1. [Zoom App Marketplace](https://marketplace.zoom.us/) にログインし、右上の「Develop」>「Build App」を選択します。
2. **「Zoom Apps」** を選んで Create します。
3. 以下の設定を入力します。
   - **Home URL**: 先ほどコピーした `https://...` のURL
   - **Redirect URL for OAuth**: （今回はOAuthを使わないゲストモードですがダミーで `https://.../auth` 等を入れておきます）
   - **Domain Allow List**: `ngrok-free.app` (お使いのngrokドメイン)
   - **Features > Zoom App**:
     - `In-Client App` をON
     - `Guest Mode` (認可不要モード) にチェックを入れる（非常に重要です）
   - **Scopes**: `zoomapp:inmeeting` などの必須スコープを追加（個人情報系は一切不要です）

## 3. Zoom クライアントでの起動確認
1. Marketplace の「Local Test」タブから「Install」または「Start」をクリックして、手元のZoomクライアントにアプリを追加します。
2. Zoomで新規ミーティングを立ち上げます。
3. 下部の「アプリ」ボタンから「蛍の光ボタン」を起動します。（サイドパネルにUIが表示されます）
4. 他の参加者（別PCやスマホ参加者）がいれば、右上の共有ボタンから **「アプリへ招待（Invite）」** を行います。
5. 招待された側は、ログインや承認画面なしで即座にアプリ（帰るボタン）が開けるはずです！
