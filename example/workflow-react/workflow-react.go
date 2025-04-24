package main

import (
	"net/http"

	"github.com/ybizeul/workflow"
)

func main() {
	wf, wfhandler, err := workflow.New("workflow.yaml", "status.yaml")
	if err != nil {
		panic(err)
	}

	defer wf.Abort()

	go func() {
		if err := wf.Start(); err != nil {
			panic(err)
		}
	}()

	http.Handle("POST /start", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		go func() {
			if err := wf.Start(); err != nil {
				panic(err)
			}
		}()
		w.WriteHeader(http.StatusOK)
	}))

	http.Handle("GET /wf", wfhandler)

	_ = http.ListenAndServe(":8080", nil)
}
