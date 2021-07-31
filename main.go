package main

import (
  "fmt"
  "log"
  "flag"
  "strings"
  "net/http"
  mqtt "github.com/eclipse/paho.mqtt.golang"
  "encoding/json"
  "sync"
)

// HTTP Basic認証のユーザ・パスワード
var g_user string
var g_pass string

// MQTTスレッドとHTTPスレッドでg_sensorの読み書きを行う際の排他処理で使用する
var g_mutex sync.Mutex

// 環境センサの最新更新値
var g_sensor Sensor

// MQTTからJSONで取得する環境センサの型
type Sensor struct {
  TimeStamp string `json:"timestamp"`                                    // *タイムスタンプ
  ErrorFlag int `json:"errorFlag"`                                       // *エラーフラグ
  WindVelocity float64 `json:"windVelocity"`                             // *風速
  WindDirection float64 `json:"windDirection"`                           // *風向き
  Temperature float64 `json:"temperature"`                               // *温度
  Humidity int `json:"humidity"`                                         // *湿度
  AirPressure int `json:"airPressure"`                                   // *気圧
  Illuminance int `json:"illuminance"`                                   // *照度
  RainLevel int `json:"rainLevel"`                                       // *レインレベル
  UltraVioletA float64 `json:"ultraVioletA"`                             // *UVA
  UltraVioletB float64 `json:"ultraVioletB"`                             // UVB
  AccelerationX float64 `json:"accelerationX"`                           // *加速度X軸
  AccelerationY float64 `json:"accelerationY"`                           // *加速度Y軸
  AccelerationZ float64 `json:"accelerationZ"`                           // *加速度Z軸
  InclinationXZ float64 `json:"inclinationXZ"`                           // *傾きXZ軸
  InclinationYZ float64 `json:"inclinationYZ"`                           // *傾きYZ軸
  MaxWindVelocity float64 `json:"maxWindVelocity"`                       // 最大風速
  DirectMaxWindVelocity float64 `json:"directMaxWindVelocity"`           // 最大風速の風向
  MaxInstWindVelocity float64 `json:"maxInstWindVelocity"`               // *最大瞬間風速
  DirectMaxInstWindVelocity float64 `json:"directMaxInstWindVelocity"`   // *最大瞬間の風向
}

// Basic認証チェック false:認証エラー
func checkAuth(r *http.Request) bool {
  user, pass, ok := r.BasicAuth()
  return ok && user == g_user && pass == g_pass
}

// "/"へのGETのハンドラ - main.html, main.jsを返す
func handleIndex(w http.ResponseWriter, r *http.Request) {
  if checkAuth(r) == false {
    w.Header().Add("WWW-Authenticate", `Basic realm="SECRET AREA"`)
    w.WriteHeader(http.StatusUnauthorized)
    http.Error(w, "Unauthorized", 401)
    return
  }

  // static/main.html, static/main.jsとして配信する方法
  //
  // ServeFileは"/index.html"というパスを"/"に変えてリダイレクトする仕様がある
  // "/"の場合はmain.htmlの中身を返すようにして/index.htmlの取得でループしないようにする
  // var path string
  // switch r.URL.Path {
  // case "/":
  //   path = "static/main.html"
  // default:
  //   path = r.URL.Path[1:]
  // }
  // w.Header().Add("Cache-Control", "no-store")
  // http.ServeFile(w, r, path)

  // HTML, JSの埋め込み文字列として配信する方法
  var text string
  switch r.URL.Path {
  case "/", "/index.html":
    text = indexHTML
  case "/static/main.js":
    text = mainJS
  default:
    text = "no file"
  }
  w.Header().Add("Cache-Control", "no-store")
  fmt.Fprintf(w, text)
}

// "/stream"へのGETのハンドラ - index.m3u8, [nnn].tsを返す
func handleVideo(w http.ResponseWriter, r *http.Request, dir string) {
  if checkAuth(r) == false {
    w.Header().Add("WWW-Authenticate", `Basic realm="SECRET AREA"`)
    w.WriteHeader(http.StatusUnauthorized)
    http.Error(w, "Unauthorized", 401)
    return
  }
  // /stream/index.m3u8という文字列からindex.m3u8というファイル名部分を取り出す
  fileName := strings.Split(r.URL.Path[1:], "/")[1]
  w.Header().Add("Cache-Control", "no-store")
  http.ServeFile(w, r, dir + "/" + fileName)
}

// "/api/sensor"へのGETのハンドラ - 環境センサの値を返す
func handleSensor(w http.ResponseWriter, r *http.Request) {
  if checkAuth(r) == false {
    w.Header().Add("WWW-Authenticate", `Basic realm="SECRET AREA"`)
    w.WriteHeader(http.StatusUnauthorized)
    http.Error(w, "Unauthorized", 401)
    return
  }
  if r.Method != http.MethodGet {
    w.WriteHeader(http.StatusMethodNotAllowed)
    http.Error(w, "NotAllowed", 405)
    return
  }
  g_mutex.Lock()
  sensor := g_sensor
  g_mutex.Unlock()
  json, err := json.Marshal(sensor)
  if err != nil {
    log.Fatal(err)
  }
  fmt.Fprintf(w, string(json))
}

// メイン関数
func main() {
  // コマンド引数解析
  var httpPort int
  flag.IntVar(&httpPort, "http", 8080, "http listen port.")
  var mqttPort int
  flag.IntVar(&mqttPort, "mqtt", 0, "mqtt listen port.")
  var videoDir string
  flag.StringVar(&videoDir, "dir", "stream", "hls video saved dir")
  flag.StringVar(&g_user, "user", "user", "basic auth username")
  flag.StringVar(&g_pass, "pass", "Iwasaki2017!", "basic auth password")
  flag.Parse()

  var f mqtt.MessageHandler = func(c mqtt.Client, m mqtt.Message) {
    var sensor Sensor
    if err := json.Unmarshal(m.Payload(), &sensor); err != nil {
      log.Fatal(err)
    }
    g_mutex.Lock()
    g_sensor = sensor
    g_mutex.Unlock()
  }

  if mqttPort != 0 {
    opts := mqtt.NewClientOptions()
    opts.AddBroker(fmt.Sprintf("tcp://localhost:%d", mqttPort))
    client := mqtt.NewClient(opts)

    if token := client.Connect(); token.Wait() && token.Error() != nil {
      log.Fatalf("Mqtt error: %s\n", token.Error())
    }

    if subscribeToken := client.Subscribe("iwasaki/location001/sensor/notify", 0, f); subscribeToken.Wait() && subscribeToken.Error() != nil {
      log.Fatalf("Mqtt error: %s\n", subscribeToken.Error())
    }
  }

  http.HandleFunc("/", handleIndex)
  http.HandleFunc("/video/", func(w http.ResponseWriter, r *http.Request) { handleVideo(w, r, videoDir) })
  http.HandleFunc("/api/sensor/", handleSensor)
  fmt.Printf("Start Server (port:%d)\n", httpPort)
  http.ListenAndServe(fmt.Sprintf(":%d", httpPort), nil)
}

const indexHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8" />
  <meta http-equiv="X-UA-Compatible" content="IE=edge" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Sensor viewer</title>
  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/bulma/0.9.3/css/bulma.min.css" />
</head>
<body>
<section class="hero is-primary is-fullheight">

<link rel=stylesheet type=text/css href="//vjs.zencdn.net/7.11.4/video-js.min.css">

<script src="//cdn.jsdelivr.net/npm/vue@2.6.10/dist/vue.min.js"></script>
<script src="//cdn.jsdelivr.net/npm/axios/dist/axios.min.js"></script>

<div id="app" class="container has-text-centered">
  <br/>

  <div class="level">
    <div class="level-left">
      <div class="subtitle is-5 level-item">
        <strong>[[ timeStampLabel ]]</strong>
      </div>
    </div>
    <div class="level-right">
      <button class="button is-info is-large is-rounded level-item" @click="fetchVideo" :disabled="isPolling">映像取得</button>
      <button class="button is-info is-large is-rounded level-item" @click="fetchImage">画像取得</button>
    </div>
  </div>

  <table class="table is-bordered is-fullwidth" style="table-layout: fixed;">
    <tbody>
      <tr>
        <td style="height: 100px;" v-for="(sensor, index) in sensors[0]" :key="index">
          <p class="has-text-primary-dark content" v-if="sensor.label.length < 5">[[ sensor.label ]]</p>
          <p class="has-text-primary-dark content is-small" v-else>[[ sensor.label ]]</p>
          <p class="has-text-primary-dark">[[ sensor.value + " "  + sensor.unit ]]</p>
        </td>
      </tr>
      <tr>
        <td style="height: 100px;" v-for="(sensor, index) in sensors[1]" :key="index">
          <p class="has-text-primary-dark">[[ sensor.label ]]</p>
          <p class="has-text-primary-dark">[[ sensor.value + " "  + sensor.unit ]]</p>
        </td>
      </tr>
    </tbody>
  </table>

  <div class="level" v-if="!video">
    <div class="level-item">
      <p class="box has-text-primary-dark">映像を取得していません</p>
    </div>
  </div>

  <div class="level">
    <div class="level-item">
      <video
        id="video"
        class="vjs-default-skin vjs-big-play-centered"
        muted
        playsinline
        data-setup='{}'
      />
    </div>
  </div>
</div>

<script src="static/main.js"></script>
<script src="//vjs.zencdn.net/7.14.1/video.min.js"></script>

</section>
</body>
</html>
`

const mainJS = `"use strict";

new Vue({
  el: "#app",
  delimiters: ["[[", "]]"], // FlaskのJinja2のtemplate記法とのバッティングを回避する

  data: {
    sensorTimeStamp: new Date(),
    sensors: [
      [ // 環境センサーデータ sensors[0] 画面1段目 digit: 小数点以下桁数
        { label: "温度", unit: "℃", key: "temperature", value: "", digit: 1 },
        { label: "湿度", unit: "％", key: "humidity", value: "", digit: 0 },
        { label: "気圧", unit: "hPa", key: "airPressure", value: "", digit: 0 },
        { label: "風速", unit: "m/s", key: "windVelocity", value: "", digit: 1 },
        { label: "風向き", unit: "deg", key: "windDirection", value: "", digit: 1 },
        { label: "最大瞬間風速", unit: "m/s", key: "maxInstWindVelocity", value: "", digit: 1 },
        { label: "最大瞬間時風向き", unit: "deg", key: "directMaxInstWindVelocity", value: "", digit: 1 },
        { label: "照度", unit: "lx", key: "illuminance", value: "", digit: 0 },
        { label: "UV", unit: "w/m2", key: "ultraVioletA", value: "", digit: 1 },
      ],
      [ // 環境センサーデータ sensors[1] 画面2段目
        { label: "レインレベル", unit: "", key: "rainLevel", value: "", digit: 0 },
        { label: "加速度X軸", unit: "G", key: "accelerationX", value: "", digit: 1 },
        { label: "加速度Y軸", unit: "G", key: "accelerationY", value: "", digit: 1 },
        { label: "加速度Z軸", unit: "G", key: "accelerationZ", value: "", digit: 1 },
        { label: "傾きXZ軸", unit: "deg", key: "inclinationXZ", value: "", digit: 1 },
        { label: "傾きYZ軸", unit: "deg", key: "inclinationYZ", value: "", digit: 1 },
        { label: "エラーフラグ", unit: "", key: "errorFlag", value: "", digit: 0 },
        { label: "", unit: "", key: "", value: "", digit: 0 },
        { label: "", unit: "", key: "", value: "", digit: 0 },
      ]
    ],
    pollingTimerObj: null, // 環境センサデータのポーリング用タイマ
    pollingPeriodTime: 1000, // 環境センサデータのポーリング間隔
    video: null, // videojsのプレイヤー
  },

  computed: {
    isPolling() {
      // ポーリングタイマオブジェクトが存在している場合はポーリング実行中である
      return this.pollingTimerObj != null;
    },

    timeStampLabel() {
      const dt = this.sensorTimeStamp;
      const yyyy = dt.getFullYear();
      const MM = dt.getMonth() + 1;
      const dd = dt.getDate();
      const HH = dt.getHours();
      const mm = dt.getMinutes();
      const week = ["日", "月", "火", "水", "木", "金", "土"][dt.getDay()];
      return yyyy + "年" + MM + "月" + dd + "日 (" + week + ") " + HH + "時" + mm + "分";
    }
  },

  methods: {
    // 映像取得
    //   - カメラのHLS動画を再生する
    //   - 環境センサ値のポーリング取得を開始する
    fetchVideo() {
      // 環境センサ値のポーリング開始
      if (this.pollingTimerObj)
        clearInterval(this.pollingTimerObj);
      this.pollingTimerObj = setInterval(this.fetchSensorData, this.pollingPeriodTime);
      // ライブラリの読み込み順がvue -> videoの順でないとうまく再生できない
      // そのためvideoオブジェクトの生成は映像取得開始時にする
      this.video = videojs('video');
      // index.m3u8を取り直す
      this.video.reset();
      this.video.src("/video/index.m3u8");
      // 再生
      this.video.play();
      this.video.one("loadedmetadata", () => {
        const w = this.video.videoWidth();
        const h = this.video.videoHeight();
        console.log("video size: " + w + "x" + h);
      });
    },

    // 画像取得
    //   - カメラのHLS動画を取得し、すぐに停止して最新画像を表示する
    //   - 環境センサ値のポーリングは行わない
    fetchImage() {
      if (this.pollingTimerObj) {
        clearInterval(this.pollingTimerObj);
        this.pollingTimerObj = null;
      }
      this.video = videojs('video');
      // index.m3u8を取り直す
      this.video.reset();
      this.video.src("/video/index.m3u8");
      // 再生
      this.video.play();
      // 再生が開始されたら停止して、画像が残るようにpauseにしておく
      this.video.one("playing", () => {
        this.video.pause();
        this.video.one("pause", () => {
          const { length, end } = this.video.seekable();
          if (length < 1) return;
          this.video.currentTime(end(length-1));
        });
      });
    },

    // 環境センサデータをサーバから取得する
    async fetchSensorData() {
      const {data: resSensor} = await axios.get("/api/sensor");
      this.sensorTimeStamp = new Date(Date.parse(resSensor.timestamp));
      for (const sensor_ of this.sensors) {
        for (const sensor of sensor_) {
          if (sensor.key) {
            const d = Math.pow(10, sensor.digit);
            const n = Math.floor(resSensor[sensor.key] * d) / d;
            sensor.value = n.toFixed(sensor.digit);
          }
        }
      }
    }
  },

  beforeDestroy() {
    if (this.video)
      this.video.dispose();
  }
});
`
