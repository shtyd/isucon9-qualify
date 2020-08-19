# ISUCON9予選　練習

## ドキュメント
### 予選レギュレーション
http://isucon.net/archives/53555007.html

### 当日マニュアル
https://github.com/isucon/isucon9-qualify/blob/master/docs/manual.md

### 講評抜粋
http://isucon.net/archives/53789931.html

サーバ環境  
ISUCON9 予選では、アリババクラウドの東京リージョンの指定したサーバインスタンスを最大3台まで起動して利用できるようにしました。

ubuntu 18.04 LTSをベースに問題のソースコード、データベース(MySQL)、リバースプロキシ(nginx)、アプリケーションサーバ(Go)が起動するOSイメージを参加者に共有し、インスタンスはこのイメージから起動する形としました。

ベンチマークは3台のうち、指定された1台のインターネット側のIPアドレスに対して検証と負荷のリクエストを送ります。1台よりも多くのインスタンスを使う場合は、最初の1台からリクエストを振り分ける必要があり、どうサーバを使い分けるかも問題を解く上で重要になります。
## 作業記録
2020/8/18  
 [1210] 初期ソースベンチ実行  
 {"pass":true,"score":1210,"campaign":0,"language":"Go","messages":["GET /new_items.json: リクエストに失敗しました（タイムアウトしました）"]}

2020/8/19  
何も変更していないが、再度webappを起動してbenchを実行するとPOST /initializeで
タイムアウトしてベンチ走行が失敗する。
``` 
2020/08/18 23:28:47 main.go:104: === initialize ===
2020/08/18 23:29:08 fails.go:72: [session.(*Session).Initialize] /Users/shota/working/isucon/isucon9-qualify/bench/session/webapp.go:217
    message("POST /initialize: リクエストに失敗しました")
``` 

昨日は失敗せずに動かせていたのに、何が違う？  
パケットキャプチャを見るとPOST /initializeのHTTP Requestから200 OKを返すまで約20秒かかってる。
/initializeで呼ばれているのはmain.goのpostInitialize()で、中でやってるのはこんなもん。
```
# DBの初期化
cmd := exec.Command("../sql/init.sh")

_, err = dbx.Exec(
		"INSERT INTO `configs` (`name`, `val`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `val` = VALUES(`val`)",
		"payment_service_url",
		ri.PaymentServiceURL,
	)

_, err = dbx.Exec(
		"INSERT INTO `configs` (`name`, `val`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `val` = VALUES(`val`)",
		"shipment_service_url",
		ri.ShipmentServiceURL,
	)
```

データベース削除 → 初期データ投入 -> webapp起動 -> paymentサービス起動 -> shipmentサービス起動 -> paymentサービス、shipmentサービス停止 -> ベンチ実行
の順番だとタイムアウトせずに通ることが分かった。paymentとshipmentを一回起動させておくことに何かがある？

```
# DB新規作成
$ cd webapp/sql/
$ cat 00_create_database.sql | mysql -u root

# テーブル初期化
$ ./init.sh

# Webapp起動
$ cd ../go/
$ ./isucari

# 別ターミナル1でpaymentサービス起動
$ ./bin/payment

# 別ターミナル2shipmentサービス起動
$ ./bin/shipment

# paymentサービスとshipmentサービスを中断(Ctrl + c)

# 別ターミナルでベンチ実行
$ ./bin/benchmarker

```

2020/8/19  
webappのリバースプロキシとしてnginx導入。  
webappのportを8000 -> 8080に変更し、
nginxのportを8000で待ち受けリクエストを8080にproxyする。 (外から見える動作は変わらない。（はず)）
