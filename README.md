# ISUCON9予選　練習

## ドキュメント
### 予選レギュレーション
http://isucon.net/archives/53555007.html

### 当日マニュアル
https://github.com/isucon/isucon9-qualify/blob/master/docs/manual.md

### ISUCARIアプリケーション仕様書
https://github.com/isucon/isucon9-qualify/blob/master/webapp/docs/APPLICATION_SPEC.md

### 外部アプリケーション仕様書
https://github.com/isucon/isucon9-qualify/blob/master/webapp/docs/EXTERNAL_SERVICE_SPEC.md

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


2020/8/20  
nginxのアクセスログをkataribeを使って解析してみる。  
合計レスポンスタイムトップ10がこう。  
/initializeは除くとして、圧倒的一位がGET /users/transactions.json 。
その後POST /buy, POST /loginが続く。
```
Top 20 Sort By Total
Count    Total     Mean   Stddev     Min   P50.0   P90.0   P95.0   P99.0     Max  2xx  3xx  4xx  5xx  TotalBytes   MinBytes  MeanBytes   MaxBytes  Request
   68  176.258   2.5920   1.8917   0.099   2.725   5.002   5.832   7.595   7.595   64    0    4    0     1261375          0      18549      23564  GET /users/transactions.json HTTP/1.1
    1   18.277  18.2770   0.0000  18.277  18.277  18.277  18.277  18.277  18.277    1    0    0    0          31         31         31         31  POST /initialize HTTP/1.1
   21   16.711   0.7958   0.8355   0.001   0.121   1.864   1.879   1.990   1.990   11    0   10    0         688          0         32         49  POST /buy HTTP/1.1
   56   15.954   0.2849   0.1637   0.111   0.215   0.555   0.620   0.659   0.659   48    0    8    0        5314         73         94        103  POST /login HTTP/1.1
    9   15.925   1.7694   0.5631   1.046   1.885   2.567   2.567   2.567   2.567    9    0    0    0      207483      23033      23053      23075  GET /new_items/60.json HTTP/1.1
    5   14.859   2.9718   0.9243   1.575   2.883   4.472   4.472   4.472   4.472    5    0    0    0      117480      23479      23496      23533  GET /new_items.json HTTP/1.1
    9   12.551   1.3946   0.5654   0.666   1.127   2.497   2.497   2.497   2.497    9    0    0    0      213051      23655      23672      23717  GET /new_items/30.json HTTP/1.1
    1    9.905   9.9050   0.0000   9.905   9.905   9.905   9.905   9.905   9.905    0    0    1    0           0          0          0          0  GET /new_items.json?created_at=1565592389&item_id=49579 HTTP/1.1
   27    9.838   0.3644   0.3239   0.003   0.214   0.853   0.937   1.123   1.123   21    0    6    0         755         13         27        106  POST /sell HTTP/1.1
    1    9.290   9.2900   0.0000   9.290   9.290   9.290   9.290   9.290   9.290    1    0    0    0       23685      23685      23685      23685  GET /new_items.json?created_at=1565592774&item_id=49965 HTTP/1.1
```

またslow requestのトップ20はこう。  
GET /new_items.json?created_at= 系が殆どを占めていて、GET /users/transactions.jsonもちらほらある。
```
TOP 37 Slow Requests
 1  18.277  POST /initialize HTTP/1.1
 2   9.905  GET /new_items.json?created_at=1565592389&item_id=49579 HTTP/1.1
 3   9.290  GET /new_items.json?created_at=1565592774&item_id=49965 HTTP/1.1
 4   8.723  GET /new_items.json?created_at=1565592776&item_id=49969 HTTP/1.1
 5   8.628  GET /new_items.json?created_at=1565592578&item_id=49772 HTTP/1.1
 6   7.644  GET /new_items.json?created_at=1565592483&item_id=49679 HTTP/1.1
 7   7.626  GET /new_items.json?created_at=1565592486&item_id=49680 HTTP/1.1
 8   7.595  GET /users/transactions.json HTTP/1.1
 9   7.023  GET /new_items.json?created_at=1565592581&item_id=49780 HTTP/1.1
10   6.892  GET /new_items.json?created_at=1565592390&item_id=49587 HTTP/1.1
11   6.770  GET /new_items.json?created_at=1565592530&item_id=49726 HTTP/1.1
12   6.570  GET /new_items.json?created_at=1565592535&item_id=49731 HTTP/1.1
13   6.452  GET /new_items.json?created_at=1565592434&item_id=49633 HTTP/1.1
14   6.270  GET /users/transactions.json HTTP/1.1
15   6.152  GET /new_items.json?created_at=1565592438&item_id=49637 HTTP/1.1
16   6.055  GET /users/transactions.json HTTP/1.1
17   5.832  GET /users/transactions.json HTTP/1.1
18   5.746  GET /users/transactions.json HTTP/1.1
19   5.644  GET /new_items.json?created_at=1565592344&item_id=49536 HTTP/1.1
20   5.635  GET /users/transactions.json HTTP/1.1
```

まずはtotalのレスポンスタイムで考えて、下記の4種類のリクエストがボトルネックになっていると分かる。  
ここのレスポンスタイムを短縮できればベンチのタイム短縮に貢献するはず。  
1. GET /users/transactions.json HTTP/1.1
2. POST /buy
3. POST /login
4. GET /new_items.json?created_at=

ただ、ベンチのスコアは、レスポンスタイムではなくて、取引が完了した商品（椅子）の価格の合計。  
これってどうやったら増えるのか・・？
```
取引が完了した商品（椅子）の価格の合計（ｲｽｺｲﾝ） - 減点 = スコア（ｲｽｺｲﾝ）
```
講評によると、
```
スコアを最大化するためには、よりたくさんの取引が完了することが必要ですが、ベンチマーカーは出品した上で各APIの回遊をし、購入処理となるため、各APIの高速化が必須となります。なお、初期の段階では商品は100ｲｽｺｲﾝでしか出品されません。
```
とのこと。ベンチは一定数の決まったリクエストが飛んでくるのではなくて、一定時間中に多くのリクエストが飛んでくるものらしい。
なのでリクエストを多くさばければスコアが高い。
```
今回のISUCONでは、例年と違い、スコアはリクエスト数から計算されるものではなく、
```
おそらくそういうこと。  
ただ今回はリクエスト数ではなくて総取引金額がスコアなので、取引数自体を多くすることと、各取引の金額を上げることでスコアを上げることが可能。  
どうすればできるか？  
取引数を増やす　-> 各APIのレスポンスタイムを短縮  
各取引の金額を上げる　-> ・・どうすれば？？？

2020/8/22  
pprofを使ってGo WebアプリのCPUプロファイルを取ってみる。  
使い方は公式のドキュメントに通りで、思ってたかなり簡単だった。  
・・、がこの情報をどう使えばよいものか？  
とりあえずmain.GetNewCategoryItemsとmain.PostLoginにかなりCPU時間使っていることはフレームグラフから読み取れる。  

2020/8/23  
スロークエリログの解析。  
slow-query.logを取得してからpt-query-digestを使って見やすくしてみる。  
見てみると↓のクエリが回数も多くて時間もかかっていることが分かる。
```
SELECT * FROM `items` WHERE `status` IN ('on_sale','sold_out') AND (`created_at` < '2019-08-12 15:36:52'  OR (`created_at` <= '2019-08-12 15:36:52' AND `id` < 49002)) ORDER BY `created_at` DESC, `id` DESC LIMIT 49\G
```

```
SELECT * FROM `items` WHERE `status` IN ('on_sale','sold_out') ORDER BY `created_at` DESC, `id` DESC LIMIT 49\G^C
```

大体ボトルネックになっていく箇所については測定で特定できたため、これからそこを改善していく。

2020/8/24  
本番環境に合わせてLinux上で環境を作っていく。  
実行環境をUbuntu18.04 4GBメモリ、2CoreのVMに変更。  
するとそれだけでスコアが1810まで上がった。(最初からこれでやればよかった）

次にNginxを入れてリバーシプロキシしただけでさらにスコアが上がった・・。
```
{"pass":true,"score":2510,"campaign":0,"language":"Go","messages":[]}
```

アクセスログのプロファイルと取り直す。
```
Top 20 Sort By Total
Count    Total    Mean  Stddev    Min  P50.0  P90.0  P95.0  P99.0    Max  2xx  3xx  4xx  5xx  TotalBytes   MinBytes  MeanBytes   MaxBytes  Request
  116  338.615  2.9191  1.6635  0.042  3.258  4.861  5.507  6.125  7.269  109    0    7    0     2179500          0      18788      30377  GET /users/transactions.json HTTP/1.1
   61   59.287  0.9719  0.8076  0.000  1.606  1.640  1.845  2.148  2.148   32    0   29    0        2154          0         35         49  POST /buy HTTP/1.1
   46   25.260  0.5491  0.4043  0.000  0.805  0.887  0.994  1.109  1.109   28    0   18    0        1846         29         40         83  POST /ship_done HTTP/1.1
   42   22.705  0.5406  0.4086  0.000  0.808  0.851  0.886  1.307  1.307   30    0   12    0        2226         29         53         61  POST /ship HTTP/1.1
   25   18.661  0.7464  0.2939  0.005  0.807  0.966  0.975  1.331  1.331   25    0    0    0         850         34         34         34  POST /complete HTTP/1.1
    2   11.985  5.9925  0.5025  5.490  6.495  6.495  6.495  6.495  6.495    1    0    1    0       21946          0      10973      21946  GET /users/transactions.json?created_at=1565575811&item_id=33007 HTTP/1.1
    2   11.128  5.5640  0.6230  4.941  6.187  6.187  6.187  6.187  6.187    2    0    0    0       43373      21675      21686      21698  GET /users/transactions.json?created_at=1565576271&item_id=33470 HTTP/1.1
   62    9.076  0.1464  0.0748  0.051  0.136  0.261  0.287  0.314  0.314   54    0    8    0        5843         73         94        107  POST /login HTTP/1.1
    2    8.980  4.4900  0.4110  4.079  4.901  4.901  4.901  4.901  4.901    2    0    0    0       43171      21574      21585      21597  GET /users/transactions.json?created_at=1565567911&item_id=25108 HTTP/1.1
    1    8.444  8.4440  0.0000  8.444  8.444  8.444  8.444  8.444  8.444    1    0    0    0          31         31         31         31  POST /initialize HTTP/1.1
```

```
TOP 37 Slow Requests
 1  8.444  POST /initialize HTTP/1.1
 2  7.269  GET /users/transactions.json HTTP/1.1
 3  6.495  GET /users/transactions.json?created_at=1565575811&item_id=33007 HTTP/1.1
 4  6.187  GET /users/transactions.json?created_at=1565576271&item_id=33470 HTTP/1.1
 5  6.125  GET /users/transactions.json HTTP/1.1
 6  6.096  GET /users/transactions.json HTTP/1.1
 7  5.714  GET /users/transactions.json HTTP/1.1
 8  5.661  GET /users/transactions.json HTTP/1.1
 9  5.507  GET /users/transactions.json HTTP/1.1
10  5.490  GET /users/transactions.json?created_at=1565575811&item_id=33007 HTTP/1.1
```
やはりこのあたりが最も遅いので、まずはここから手を着けていく。
GET /users/transactions.json HTTP/1.1  
GET /users/transactions.json?created_at=

次にSlow-queryログをlong_query_time = 2で取得してみたが、ログが出ない。  
long_query_time = 1にしても出なくなった。  
先頭のログ開始の表記などは都度書き込まれているので、書き込み失敗している訳ではないはず。  
インフラ変えただけでスロークエリも無くなった・・？　　

2020/8/25  
pprofでgoアプリのプロファイルを取る。  
あまりmacと変わらず、とりあえずmain.GetNewCategoryItemsとmain.PostLoginにかなりCPU時間使っていることは分かる。

2020/8/26  
キャンペーン = 1に設定してみると、ベンチのリクエストが増えて下記エラーが大量に出て失敗になった。
```
 main.go:435: Error 1040: Too many connections
```

max_connectionを上げればこのエラーが出なくなったがベンチはpassにならない・・。

2020/8/27  
too many open filesのエラーがたくさん出ていた。
```
2020/08/27 21:50:59 server.go:3095: http: Accept error: accept tcp [::]:7000: accept4: too many open files; retrying in 5ms
```

nginxて下記設定いれてみても改善せず。  
worker_rlimit_nofile  200000;

ulimitを見てみるとデフォルトの1024しかなかったので、100000くらいに設定すると、
too many open filesのエラーもなくなり、スコアも伸びた。
```
$ ulimit -n
1024

$ ulimit -Sn 100000
```
ファイルディスクリプタの上限を上げてやる必要があった。
too many open files -> ファイルディスクリプタの上限を上げる。(nginxとulimit)
