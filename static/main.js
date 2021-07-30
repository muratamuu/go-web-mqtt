"use strict";

new Vue({
  el: "#app",
  delimiters: ["[[", "]]"], // FlaskのJinja2のtemplate記法とのバッティングを回避する

  data: {
    sensorTimeStamp: new Date(),
    sensors: [
      [ // 環境センサーデータ sensors[0] 画面1段目 digit: 小数点以下桁数
        { label: "温度", unit: "℃", key: "temperature", value: "", digit: 1 },
        { label: "湿度", unit: "%", key: "humidity", value: "", digit: 0 },
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
      return `${yyyy}年${MM}月${dd}日 (${week}) ${HH}時${mm}分`;
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
      this.video.src("/stream/index.m3u8");
      // 再生
      this.video.play();
      this.video.one("loadedmetadata", () => {
        const w = this.video.videoWidth();
        const h = this.video.videoHeight();
        console.log(`video size: ${w}x${h}`);
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
      this.video.src("/stream/index.m3u8");
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
      const {data:{sensor: resSensor}} = await axios.get("/api/sensor");
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
