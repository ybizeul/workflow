package workflow

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// make_socket creates a new webserver for workflow wf and handler fn.
// It returns the websocket connection, and a function to start the workflow
// The websocket connects immediatly, but the workflow is started only when
// the start function is called.
func make_socket(t *testing.T, wf *Workflow, fn http.Handler) (*websocket.Conn, func()) {

	s := httptest.NewServer(fn)

	u := "ws" + strings.TrimPrefix(s.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), u, nil)
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan bool)
	go func() {
		defer func() {
			s.Close()
		}()
		<-start
		err := wf.Start()
		if err != nil {
			slog.Error("task failed", "error", err)
		}
	}()

	return conn, func() { start <- true }
}

// Test just the workflow proper execution
func TestWorkflow(t *testing.T) {
	wf, _, err := New("test_data/test.yaml", "test_data/status.json")
	if err != nil {
		t.Fatal(err)
	}

	err = wf.Start()
	if err != nil {
		t.Fatal(err)
	}
}

// Test workflow result data
func TestWorkflowWS(t *testing.T) {
	t.Cleanup(func() {
		os.Remove("test_data/status.json")
	})

	wf, fn, err := New("test_data/test.yaml", "test_data/status.json")
	if err != nil {
		t.Fatal(err)
	}

	conn, start := make_socket(t, wf, fn)

	start()

	got := ""
	var previous string
	for {
		var message Status

		err := wsjson.Read(context.Background(), conn, &message)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				break
			}
			slog.Error("error while reading", "error", err)
			break
		}
		if message.LastMessage != "" && message.LastMessage != previous {
			got += message.LastMessage + "\n"
		}
		previous = message.LastMessage
	}

	want := "Some Data\nvar1\nvar2\ntask2 finished\nvar1\nvar2\nvar1\nvar2\n"

	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

// Test that connecting late to the websocket only returns current status
func TestWorkflowLateWS(t *testing.T) {
	t.Cleanup(func() {
		os.Remove("test_data/status.json")
	})

	// Initialize Wokflow
	wf, fn, err := New("test_data/test.yaml", "test_data/status.json")
	if err != nil {
		t.Fatal(err)
	}

	// Start it right away in the background
	go func() {
		err := wf.Start()
		if err != nil {
			slog.Error("task failed", "error", err)
		}
	}()

	// Wait 5 seconds and miss the first entry
	time.Sleep(3 * time.Second)

	// Start test web server
	r := httptest.NewServer(fn)
	defer r.Close()

	// Connect to it
	u := "ws" + strings.TrimPrefix(r.URL, "http")

	conn, _, err := websocket.Dial(context.Background(), u, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Log messages
	var message Status

	var got, previous string

	for wsjson.Read(context.Background(), conn, &message) == nil {
		if message.LastMessage != previous {
			got += message.LastMessage + "\n"
		}
		previous = message.LastMessage
	}

	// Reading is done, test output
	want := "var1\nvar2\ntask2 finished\nvar1\nvar2\nvar1\nvar2\n"

	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

// Test that early termination of the connection behaves as expected with
// proper connections cleanup
func TestWorkflowEarlyTerminationWS(t *testing.T) {
	t.Cleanup(func() {
		os.Remove("test_data/status.json")
	})

	wf, fn, err := New("test_data/test.yaml", "test_data/status.json")
	if err != nil {
		t.Fatal(err)
	}

	conn, start := make_socket(t, wf, fn)

	var message Status

	got := ""

	start()

	for {
		err = wsjson.Read(context.Background(), conn, &message)
		if err != nil {
			slog.Error("error while reading", "error", err)
		}
		if message.LastMessage != "" {
			break
		}
	}

	err = conn.Close(websocket.StatusNormalClosure, "")
	if err != nil {
		slog.Error("error while closing", "error", err)
	}

	want := "Some Data\n"

	got += message.LastMessage + "\n"

	wf.Abort()

	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}

	// Check the disconnected socket has been cleaned up
	if len(wf.ws) != 0 {
		t.Fatalf("websocket not closed")
	}
}

// Test that a group set for skipping is effectively skipped
func TestGroupSkip(t *testing.T) {
	t.Cleanup(func() {
		os.Remove("test_data/status.json")
	})

	wf, fn, err := New("test_data/test-skip.yaml", "test_data/status.json")
	if err != nil {
		t.Fatal(err)
	}

	conn, start := make_socket(t, wf, fn)

	var status Status

	start()
	for {
		err := wsjson.Read(context.Background(), conn, &status)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				break
			}
			slog.Error("error while reading", "error", err)
			break
		}
		if status.LastMessage != "group2" {
			t.Fatalf("group1 not skipped")
		}
	}
}

func TestOutputAndProgress(t *testing.T) {
	t.Cleanup(func() {
		os.Remove("test_data/status.json")
	})

	wf, fn, err := New("test_data/test-output.yaml", "test_data/status.json")
	if err != nil {
		t.Fatal(err)
	}

	conn, start := make_socket(t, wf, fn)

	start()

	statuses := []*struct {
		Percent int
		Message string
		Done    bool
	}{
		{Percent: 0, Message: "task1"},
		{Percent: 20, Message: "task1"},
		{Percent: 40, Message: "task1"},
		{Percent: 50, Message: "task1"},
		{Percent: 50, Message: "task2"},
		{Percent: 60, Message: "task2"},
		{Percent: 80, Message: "task2"},
		{Percent: 100, Message: "task2"},
	}
	for i := range statuses {
		status := statuses[i]
		expectedPercent := status.Percent
		expectedMessage := status.Message
		for {
			var s Status
			err := wsjson.Read(context.Background(), conn, &s)
			if err != nil {
				if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
					goto end
				}
				slog.Error("error while reading", "error", err)
				break
			}
			matchPercent := expectedPercent == s.Percent
			matchMessage := expectedMessage == s.LastMessage
			if matchPercent && matchMessage {
				status.Done = true
				goto next
			}
		}
	next:
	}
end:
	for i := range statuses {
		if !statuses[i].Done {
			t.Fatalf("status %+v not received", statuses[i])
		}
	}
}

// func TestDoc(t *testing.T) {
// 	t.Cleanup(func() {
// 		os.Remove("test_data/status.json")
// 	})

// 	wf, fn, err := New("test_data/test-doc.yaml", "test_data/status.json")
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	conn, start := make_socket(t, wf, fn)

// 	var status Status

// 	start()
// 	for {
// 		err := wsjson.Read(context.Background(), conn, &status)
// 		if err != nil {
// 			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
// 				break
// 			}
// 			slog.Error("error while reading", "error", err)
// 			break
// 		}
// 		slog.Info("status", "status", status)
// 	}
// }
