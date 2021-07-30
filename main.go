package main

import (
  "net/http"
)

const (
  authUser = "user"
  authPass = "Iwasaki2017!"
)

func checkAuth(r *http.Request) bool {
  user, pass, ok := r.BasicAuth()
  return ok && user == authUser && pass == authPass
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
  if checkAuth(r) == false {
    w.Header().Add("WWW-Authenticate", `Basic realm="SECRET AREA"`)
    w.WriteHeader(http.StatusUnauthorized)
    http.Error(w, "Unauthorized", 401)
    return
  }
  path := r.URL.Path[1:]
  if path == "" {
    path = "main.html"
  }
  w.Header().Add("Cache-Control", "no-store")
  http.ServeFile(w, r, "static/" + path)
}

func main() {
  http.HandleFunc("/", handleIndex)
  http.ListenAndServe(":8080", nil)
}

