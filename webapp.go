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
  "os"
  "time"
  "path/filepath"
)

// HTTP Basic認証のユーザ・パスワード
var g_user string
var g_pass string

// MQTTスレッドとHTTPスレッドでg_sensorの読み書きを行う際の排他処理で使用する
var g_mutex sync.Mutex

// 環境センサの最新更新値
var g_sensor = Sensor{TimeStamp: time.Now().Format(time.RFC3339)}

// MQTTからJSONで取得する環境センサの型
type Sensor struct {
  TimeStamp string `json:"timestamp"`                                    // *タイムスタンプ
  ErrorFlag float64 `json:"errorFlag"`                                   // *エラーフラグ
  WindVelocity float64 `json:"windVelocity"`                             // *風速
  WindDirection float64 `json:"windDirection"`                           // *風向き
  Temperature float64 `json:"temperature"`                               // *温度
  Humidity float64 `json:"humidity"`                                     // *湿度
  AirPressure float64 `json:"airPressure"`                               // *気圧
  Illuminance float64 `json:"illuminance"`                               // *照度
  RainLevel float64 `json:"rainLevel"`                                   // *レインレベル
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
func handleIndex(w http.ResponseWriter, r *http.Request, dir string) {
  if checkAuth(r) == false {
    w.Header().Add("WWW-Authenticate", `Basic realm="SECRET AREA"`)
    w.WriteHeader(http.StatusUnauthorized)
    http.Error(w, "Unauthorized", 401)
    return
  }
  // http.ServeFileは"/index.html"というパスは"/"に変えてリダイレクトしてしまう仕様のようである。
  // 困ったことに、"/index.html"を返そうとすると"/index.html" -> "/" -> "/index.html"... となり返せない
  // したがって、"/index.html" -> "/" -> "/main.html" となるように "/"のパスでmain.htmlを返すようにしている
  var path string
  switch r.URL.Path {
  case "/":
    path = "main.html"
  default:
    path = r.URL.Path[1:]
  }
  w.Header().Add("Cache-Control", "no-store")
  http.ServeFile(w, r, dir + "/" + path)
}

// "/video"へのGETのハンドラ - index.m3u8, [nnn].tsを返す
func handleVideo(w http.ResponseWriter, r *http.Request, dir string) {
  if checkAuth(r) == false {
    w.Header().Add("WWW-Authenticate", `Basic realm="SECRET AREA"`)
    w.WriteHeader(http.StatusUnauthorized)
    http.Error(w, "Unauthorized", 401)
    return
  }
  // /video/index.m3u8というパス文字列からindex.m3u8というファイル名部分を取り出す
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
  g_mutex.Lock()
  sensor := g_sensor
  g_mutex.Unlock()
  json, err := json.Marshal(sensor)
  if err != nil {
    log.Fatal(err)
  }
  fmt.Fprintf(w, string(json))
}

// コマンド引数
type Args struct {
  httpPort int
  mqttPort int
  authUser string
  authPass string
  videoDir string
  htmlDir  string
}

// コマンド引数解析
func parseArgs() Args {
  args := Args{}
  flag.IntVar(&args.httpPort, "http", 8080, "http listen port.")
  flag.IntVar(&args.mqttPort, "mqtt", 0, "mqtt listen port. (please set 21883)")
  flag.StringVar(&args.authUser, "user", "user", "basic auth username")
  flag.StringVar(&args.authPass, "pass", "Iwasaki2017!", "basic auth password")
  flag.StringVar(&args.videoDir, "dir", "/var/video", "hls video saved dir")
  exe, _ := os.Executable()
  htmlDir := filepath.Dir(exe) + "/static"
  flag.StringVar(&args.htmlDir, "html", htmlDir, "html asset dir")
  flag.Parse()
  return args
}

// メイン関数
func main() {
  // コマンド引数解析
  args := parseArgs()
  g_user = args.authUser
  g_pass = args.authPass

  // videoディレクトリ作成
  if _, err := os.Stat(args.videoDir); os.IsNotExist(err) {
    if err := os.MkdirAll(args.videoDir, 0777); err != nil {
      log.Fatal(err)
    }
  }

  var f mqtt.MessageHandler = func(c mqtt.Client, m mqtt.Message) {
    var sensor Sensor
    if err := json.Unmarshal(m.Payload(), &sensor); err != nil {
      log.Fatal(err)
    }
    g_mutex.Lock()
    g_sensor = sensor
    g_mutex.Unlock()
  }

  if args.mqttPort != 0 {
    opts := mqtt.NewClientOptions()
    opts.AddBroker(fmt.Sprintf("tcp://localhost:%d", args.mqttPort))
    client := mqtt.NewClient(opts)

    if token := client.Connect(); token.Wait() && token.Error() != nil {
      log.Fatalf("Mqtt error: %s\n", token.Error())
    }

    if subscribeToken := client.Subscribe("iwasaki/location001/sensor/notify", 0, f);
      subscribeToken.Wait() && subscribeToken.Error() != nil {
      log.Fatalf("Mqtt error: %s\n", subscribeToken.Error())
    }
  }

  http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    handleIndex(w, r, args.htmlDir)
  })
  http.HandleFunc("/video/", func(w http.ResponseWriter, r *http.Request) {
    handleVideo(w, r, args.videoDir)
  })
  http.HandleFunc("/api/", handleSensor)
  fmt.Printf("Start Server (port:%d)\n", args.httpPort)
  http.ListenAndServe(fmt.Sprintf(":%d", args.httpPort), nil)
}

