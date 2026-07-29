package main

import (
	"embed"
	"encoding/json"
	"flag"
	"io/fs"
	"log"
	"net/http"

	feel "github.com/binary141/FEEL.go"
)

//go:embed static/index.html
var static embed.FS

type evalRequest struct {
	Expression string `json:"expression"`
	Context    string `json:"context"`
}

type evalResponse struct {
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

func evalHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req evalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	result, err := feel.EvalString(req.Expression, req.Context)
	if err != nil {
		json.NewEncoder(w).Encode(evalResponse{Error: err.Error()})
		return
	}

	json.NewEncoder(w).Encode(evalResponse{Result: result})
}

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	flag.Parse()

	sub, err := fs.Sub(static, "static")
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/", http.FileServer(http.FS(sub)))
	http.HandleFunc("/api/eval", evalHandler)

	log.Printf("listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
