package main

import (
	"log/slog"
	"net/http"

	"github.com/ybizeul/workflow"
)

func main() {
	wf, wfhandler, err := workflow.New("workflow.yaml", "status.yaml")
	if err != nil {
		panic(err)
	}

	defer wf.Abort()

	http.Handle("POST /start", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		go func() {
			wf.Reset()
			if err := wf.Start(); err != nil {
				slog.Error("Workflow ended", "err", err)
			}
		}()
		w.WriteHeader(http.StatusOK)
	}))

	http.Handle("POST /stop", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wf.Abort()
		w.WriteHeader(http.StatusOK)
	}))

	http.Handle("GET /wf", wfhandler)

	_ = http.ListenAndServe(":8080", nil)
}
