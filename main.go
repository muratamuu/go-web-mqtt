package main

import (
  "net/http"
  "text/template"
)

var templates = make(map[string]*template.Template)

func loadTemplate(name string) *template.Template {
  t, err := template.ParseFiles(
    "templates/" + name + ".html",
    "templates/_header.html",
    "templates/_footer.html",
  )
  if err != nil {
    panic(err)
  }
  return t
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
  if err := templates["index"].Execute(w, nil); err != nil {
    panic(err)
  }
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
  if err := templates["login"].Execute(w, nil); err != nil {
    panic(err)
  }
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
  panic("")
}

func main() {
  templates["index"] = loadTemplate("index");
  templates["login"] = loadTemplate("login");
  http.HandleFunc("/", handleIndex)
  http.HandleFunc("/login", handleLogin)
  http.HandleFunc("/logout", handleLogout)
  http.Handle("/static/", http.FileServer(http.Dir(".")))
  http.ListenAndServe(":8080", nil)
}

