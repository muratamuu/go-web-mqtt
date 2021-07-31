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
  // ServeFileは"/index.html"というパスを"/"に変えてリダイレクトする仕様がある
  // "/"の場合はmain.htmlの中身を返すようにして/index.htmlの取得でループしないようにする
  var path string
  switch r.URL.Path {
  case "/":
    path = "static/main.html"
  default:
    path = r.URL.Path[1:]
  }
  w.Header().Add("Cache-Control", "no-store")
  http.ServeFile(w, r, path)
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

