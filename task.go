package workflow

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"syscall"
)

type Task struct {
	Id     string `json:"id"`
	Cmd    string `json:"cmd"`
	Weight int    `json:"weight"`
	Exits  bool   `json:"exits"`

	Started  bool    `json:"started"`
	Finished bool    `json:"finished"`
	Percent  float64 `json:"percent"`
	Error    string  `json:"error"`

	cmd        *exec.Cmd
	cmd_Stdout io.WriteCloser
	cmd_Stderr io.WriteCloser
	cmd_WFout  io.WriteCloser

	Stdout io.ReadCloser `json:"-"`
	Stderr io.ReadCloser `json:"-"`
	Wfout  io.ReadCloser `json:"-"`
}

func NewTask(y map[string]any) (*Task, error) {
	id, ok := y["id"].(string)
	if !ok {
		return nil, WorkflowErrorTaskMissingId
	}

	cmd, ok := y["cmd"].(string)
	if !ok || cmd == "" {
		return nil, WorkflowErrorTaskMissingCommand
	}

	var weight int
	weight, _ = y["weight"].(int)

	if weight == 0 {
		weight = 1
	}

	var exits bool
	exits, _ = y["exits"].(bool)

	return &Task{
		Id:     id,
		Cmd:    cmd,
		Weight: weight,
		Exits:  exits,
	}, nil
}

func (t *Task) Run(ctx context.Context, cwd string) error {

	cmd := exec.Command("/bin/bash", "-c", `
	function output() {
		[ -p "$WFOUT" ] && echo "output:: $*" > "$WFOUT"
	}
	function progress() {
		[ -p "$WFOUT" ] && echo "progress:: $*" > "$WFOUT"
	}
	function error() {
		[ -p "$WFOUT" ] && echo "error:: $*" > "$WFOUT"
	}
	`+t.Cmd)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	cmd.Dir = cwd
	t.cmd = cmd

	// Add variables to the environment
	vars, ok := ctx.Value(contextKeyVars).(map[string]string)
	if ok {
		for k, v := range vars {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Connect Stdout & Stderr
	if t.Stdout == nil {
		slog.Debug("setting Stdout to os.Stdout")
		cmd.Stdout = os.Stdout
	} else {
		slog.Debug("setting Stdout to cmd_Stdout")
		cmd.Stdout = t.cmd_Stdout
	}

	if t.Stderr == nil {
		slog.Debug("setting Stderr to os.Stderr")
		cmd.Stderr = os.Stderr
	} else {
		slog.Debug("setting Stderr to cmd_Stderr")
		cmd.Stderr = t.cmd_Stderr
	}

	// Create local fifo for messages

	wfout_dir_path, err := os.MkdirTemp("", "workflow.*")
	if err != nil {
		return nil
	}
	wfout_path := path.Join(wfout_dir_path, ".task")
	defer os.RemoveAll(wfout_dir_path)
	err = syscall.Mknod(wfout_path, syscall.S_IFIFO|0666, 0)
	if err != nil {
		return err
	}

	cmd.Env = append(cmd.Env, fmt.Sprintf("WFOUT=%s", wfout_path))

	block_output := make(chan struct{})
	block_start := make(chan struct{})

	var outputf *os.File

	go func() {
		outputf, err = os.OpenFile(wfout_path, os.O_RDWR, 0)
		if err != nil {
			slog.Error("unable to open fifo", "error", err)
			close(block_start)
			return
		}
		close(block_start)
		defer func() {
			err := outputf.Close()
			if err != nil {
				slog.Error("error while closing fifo", "error", err)
			}
		}()

		rd := bufio.NewReader(outputf)
		for {
			s, err := rd.ReadString('\n')
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				slog.Error("error while reading fifo", "error", err)
				break
			}

			if s == "end::\n" {
				close(block_output)
				break
			}

			_, err = os.Stdout.WriteString(s)
			if err != nil {
				slog.Error("error while writing to Stdout", "error", err)
				break
			}

			if t.cmd_WFout != nil {
				_, err = t.cmd_WFout.Write([]byte(s))
				if err != nil {
					slog.Error("error while writing to WFout", "error", err)
					break
				}
			}
		}
	}()

	<-block_start
	err = cmd.Start()
	if err != nil {
		slog.Error("error while starting command", "error", err)
		if t.Error == "" {
			t.Error = err.Error()
		}
	}
	cmdErr := cmd.Wait()

	_, err = outputf.WriteString("end::\n")
	if err != nil {
		return err
	}

	<-block_output

	if t.cmd_WFout != nil {
		err := t.cmd_WFout.Close()
		if err != nil {
			return err
		}
	}

	return cmdErr
}

func (t *Task) Abort() error {
	slog.Warn("aborting task", "task", t.Id)
	pgid, err := syscall.Getpgid(t.cmd.Process.Pid)
	if err == nil {
		_ = syscall.Kill(-pgid, 15) // note the minus sign
	}
	//err := t.cmd.Process.Signal(syscall.SIGINT)
	//pid := t.cmd.Process.Pid
	//err := t.cmd.Process.Kill()
	if err != nil {
		return err
	}
	return nil
}

func (t *Task) StdoutPipe() (io.ReadCloser, error) {
	if t.Stdout != nil {
		return nil, errors.New("stdout already set")
	}
	pr, pw := io.Pipe()
	t.cmd_Stdout = pw
	t.Stdout = pr
	return pr, nil
}

func (t *Task) StderrPipe() (io.ReadCloser, error) {
	if t.Stderr != nil {
		return nil, errors.New("stderr already set")
	}
	pr, pw := io.Pipe()
	t.cmd_Stderr = pw
	t.Stderr = pr
	return pr, nil
}

func (t *Task) WfoutPipe() (io.ReadCloser, error) {
	if t.Wfout != nil {
		return nil, errors.New("wfout already set")
	}
	pr, pw := io.Pipe()
	t.cmd_WFout = pw
	t.Wfout = pr
	return pr, nil
}
