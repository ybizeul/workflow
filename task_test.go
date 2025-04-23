package workflow

import (
	"bufio"
	"context"
	"log/slog"
	"testing"
)

func TestTaskNoOutput(t *testing.T) {
	task, err := NewTask(map[string]any{
		"id": "tast-task",
		"cmd": `echo "sample text"
		output sample output`,
	})

	if err != nil {
		t.Fatal(err)
	}

	err = task.Run(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestTaskOutput(t *testing.T) {
	task, err := NewTask(map[string]any{
		"id": "tast-task",
		"cmd": `echo "sample text"
		output sample output`,
	})

	if err != nil {
		t.Fatal(err)
	}

	wfout, err := task.WfoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	output := ""
	go func() {
		rd := bufio.NewReader(wfout)
		for {
			s, err := rd.ReadString('\n')
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				slog.Error("error while reading fifo", "error", err)
				break
			}
			output += s
		}
	}()

	stdout, err := task.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	stdoutstr := ""
	go func() {
		rd := bufio.NewReader(stdout)
		for {
			s, err := rd.ReadString('\n')
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				slog.Error("error while reading fifo", "error", err)
				break
			}
			stdoutstr += s
		}
	}()

	err = task.Run(context.Background(), ".")
	if err != nil {
		t.Fatal(err)
	}

	want := "output:: sample output\n"
	if output != want {
		t.Fatalf("want %q, got %q", want, output)
	}

	want = "sample text\n"
	if stdoutstr != want {
		t.Fatalf("want %q, got %q", want, output)
	}
}
