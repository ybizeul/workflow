package workflow

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"gopkg.in/yaml.v3"
)

type Workflow struct {
	Status Status // Status of the current workflow

	workflowPath string // Path to the workflow definition file
	statusPath   string // Path to the status persistent state

	ctx         context.Context
	cancel      context.CancelFunc
	currentTask *Task
	ws          []*websocket.Conn

	sync.Mutex
}

// Status all the informations related to the workflow run.
type Status struct {
	// Definition is a copy of the original workflow definition
	// For example, if you defined a multi step workflow and the original
	// definition file is changed, this will not be updated.
	Definition map[string]any `json:"definition"`

	// Vars contains all the values for variables defined in the workflow
	Vars map[string]string `json:"vars"`

	// Groups contains all the groups defined in the workflow, which in turn
	// contains all the tasks.
	Groups []*Group `json:"groups"`

	Started  bool `json:"started"`  // Workflow has been started
	Finished bool `json:"finished"` // Workflow has finished
	Percent  int  `json:"percent"`  // Workflow progress in percent, assuming all tasks have weights defined

	LastMessage string `json:"lastMessage"` // The last message returned by a task using `output`

	CurrentGroup string `json:"currentGroup"` // Currently running group
	CurrentTask  string `json:"currentTask"`  // Currently running task

	Error string `json:"error,omitempty"` // Last error
}

// New returns a new Workflow with definition at definitionPath and status file
// to be written at statusFilePath as well as a http.HandlerFunc for handling
// websocket connections.
func New(definitionFilePath string, statusFilePath string) (*Workflow, http.Handler, error) {
	result := &Workflow{
		workflowPath: definitionFilePath,
		statusPath:   statusFilePath,
	}

	// If a status file exists, probably from a previous run, load it
	// and use it to initialize the workflow
	// Otherwise, initialize the workflow from the definition file
	// and create a new status file
	b, err := os.ReadFile(result.statusPath)
	if err != nil {
		if os.IsNotExist(err) {
			err := result.initialize()
			if err != nil {
				return nil, nil, err
			}
		} else {
			return nil, nil, err
		}
	} else {
		// Status file exists, load the status from it
		err := json.Unmarshal(b, &result.Status)
		if err != nil {
			return nil, nil, err
		}
	}

	// Create websocket handler function that promotes the request to a
	// websocket and sends current status to it.
	websocketHandlerFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			slog.Error("unable to create websocket", "error", err)
			return
		}
		ctx := conn.CloseRead(r.Context())

		result.ws = append(result.ws, conn)

		err = result.writeSockets()
		if err != nil {
			slog.Error("unable to write initial status to websocket", "error", err)
			return
		}

		<-ctx.Done()

		conn.Close(websocket.StatusNormalClosure, "")

		// Remove connection from list
		for i := range result.ws {
			if result.ws[i] == conn {
				result.ws = append(result.ws[:i], result.ws[i+1:]...)
				break
			}
		}
	})

	return result, websocketHandlerFunc, nil
}

type contextKey struct {
	name string
}

func (k *contextKey) String() string { return "workflow context value " + k.name }

var contextKeyVars = contextKey{"vars"}

func (w *Workflow) initialize() error {
	w.Status = Status{
		Definition: map[string]any{},
	}

	// Read workflow definition from YAML
	f, err := os.ReadFile(w.workflowPath)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(f, w.Status.Definition)
	if err != nil {
		return err
	}

	groups_definition, ok := w.Status.Definition["groups"].([]any)
	if !ok {
		return WorkflowErrorNoGroups
	}

	w.Status.Groups, err = loadGroups(groups_definition)
	if err != nil {
		return err
	}
	return nil
}

func (w *Workflow) loadVars(definitions map[string]any) (map[string]string, error) {
	result := map[string]string{}

	for k, v := range definitions {
		command, ok := v.(string)
		if !ok {
			return nil, WorkflowErrorInvalidVars
		}
		cmd := exec.Command("sh", "-c", command)
		cmd.Dir = path.Dir(w.workflowPath)

		out, err := cmd.Output()
		if err != nil {
			slog.Error("error while initializing variables", "error", err)
			return nil, fmt.Errorf("failed to initialize variable %s: %w", k, err)
		}
		value := strings.TrimSpace(string(out))
		result[k] = value
	}

	return result, nil
}

func loadGroups(definitions []any) ([]*Group, error) {
	result := []*Group{}
	for i := range definitions {
		group, err := newGroup(definitions[i].(map[string]any))
		if err != nil {
			return nil, err
		}
		result = append(result, group)
	}

	return result, nil
}

// Start starts the workflow execution and returns any error encountered
func (w *Workflow) Start() error {
	// Close the websocket when done
	defer func() {
		for _, ws := range w.ws {
			if err := ws.Close(websocket.StatusNormalClosure, ""); err != nil {
				slog.Error("error closing websocket", "error", err)
			}
		}
	}()

	// Load vars values
	var err error
	if w.Status.Vars == nil {
		if vars, ok := w.Status.Definition["vars"]; ok {
			vars, ok := vars.(map[string]any)
			if !ok {
				return WorkflowErrorInvalidVars
			}
			w.Status.Vars, err = w.loadVars(vars)
			if err != nil {
				return err
			}
		}
	}

	groups := w.Status.Groups

	// Load group skip values
	for g := range groups {
		group := groups[g]

		if group.skip_cmd == "" {
			continue
		}

		// Check if group should be skipped
		cmd := exec.Command("bash", "-c", group.skip_cmd)

		// Setup environment with variables values
		for k, v := range w.Status.Vars {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}

		// Commands are always executed in the workflow directory
		cmd.Dir = path.Dir(w.workflowPath)

		err = cmd.Run()
		if err == nil {
			group.Skip = true
		}
	}

	w.ctx, w.cancel = context.WithCancel(context.WithValue(context.Background(), contextKeyVars, w.Status.Vars))

	w.Status.Started = true
	err = w.writeStatus()
	if err != nil {
		return err
	}

	defer func() {
		w.Status.Finished = true
		_ = w.writeStatus()
		_ = w.writeSockets()
		os.Remove(w.statusPath)
	}()

	seeking := false
	if w.Status.CurrentTask != "" {
		seeking = true
	}

	slog.Debug("starting workflow", "seeking", seeking, "status", w.Status, "groups", groups)
	for _, group := range groups {
		if group.Skip {
			slog.Debug("normal skipping group", "group", group.Id)
			continue
		}

		if seeking && group.Id != w.Status.CurrentGroup {
			for i := range group.Tasks {
				group.Started = true
				group.Finished = true
				group.Tasks[i].Started = true
				group.Tasks[i].Finished = true
			}
			slog.Debug("skipping group (not current group)", "group", group.Id)
			continue
		}
		w.Status.CurrentGroup = group.Id
		group.Started = true
		for _, task := range group.Tasks {
			slog.Debug("starting task", "task", task)
			// Handle cancellation
			if err := w.ctx.Err(); err != nil {
				slog.Warn("workflow aborted", "error", err)
				task.Error = err.Error()
				group.Error = err.Error()
				w.Status.Error = err.Error()
				return nil
			}

			if seeking && task.Id != w.Status.CurrentTask {
				task.Started = true
				task.Finished = true
				slog.Debug("skipping task (not current task)", "task", task)
				continue
			}

			if seeking {
				seeking = false
				if task.Exits {
					task.Started = true
					task.Finished = true
					continue
				}
			}
			w.Status.CurrentTask = task.Id
			w.currentTask = task
			err = w.writeStatus()
			if err != nil {
				return err
			}

			wfout, err := task.wfoutPipe()
			if err != nil {
				return err
			}

			go func() {
				defer wfout.Close()
				rd := bufio.NewReader(wfout)
				for {
					s, err := rd.ReadString('\n')
					if err != nil {
						if err == io.EOF || errors.Is(err, os.ErrClosed) {
							slog.Debug("wfout closed, exiting reader loop")
							break
						}
						slog.Error("error while reading fifo", "error", err)
						break
					}

					switch {
					case strings.HasPrefix(s, "progress:: "):
						s = strings.TrimPrefix(s, "progress:: ")
						s = strings.TrimSpace(s)
						progress, err := strconv.ParseFloat(s, 64)
						if err != nil {
							slog.Error("unable to parse progress", "error", err)
							continue
						}
						w.currentTask.Percent = progress
					case strings.HasPrefix(s, "output:: "):
						s = strings.TrimPrefix(s, "output:: ")
						s = strings.TrimSpace(s)

						w.Status.LastMessage = s
						group.LastMessage = s
					case strings.HasPrefix(s, "error:: "):
						s = strings.TrimPrefix(s, "error:: ")
						s = strings.TrimSpace(s)

						task.Error = s
						group.Error = s
						w.Status.Error = s
					}

					err = w.writeStatus()
					if err != nil {
						slog.Error("unable to write status", "error", err)
					}
					err = w.writeSockets()
					if err != nil {
						slog.Error("unable to write status to socket", "error", err)
					}
				}
			}()
			task.Started = true

			slog.Debug("running task", "task", task)
			err = task.run(w.ctx, path.Dir(w.workflowPath))

			if err != nil {
				if task.Error == "" {
					task.Error = err.Error()
					group.Error = err.Error()
					w.Status.Error = err.Error()
					w.Status.Finished = true
				}
				_ = w.writeStatus()
				_ = w.writeSockets()
				return err
			}
			if !task.Exits {
				task.Finished = true
			}
			slog.Debug("task ended", "task", task)

			_ = w.writeStatus()
			_ = w.writeSockets()

			if task.Exits {
				os.Exit(128)
			}
		}
		group.Finished = true
		slog.Debug("group ended", "task", group)

		_ = w.writeStatus()
		_ = w.writeSockets()
	}

	// time.Sleep(1 * time.Second)
	return nil
}

// Reset is used to set the workflow to a clean state, and is only possible
// if execution is finished. It can be used to run a workflow again without
// having to create a new instance.
func (w *Workflow) Reset() error {
	if !w.Status.Finished {
		return WorkflowErrorNotFinished
	}
	err := w.initialize()
	if err != nil {
		return err
	}
	return nil
}

// Abort kills current tasks and stops workflow execution
func (w *Workflow) Abort() {
	w.cancel()
	err := w.currentTask.abort()
	if err != nil {
		slog.Error("unable to abort task", "error", err)
	}
}

// Continue is used instead of [Start] when a status file already exists after
// a previous unfinished run. If the statufile does not exists, it will
// return a file not found error.
func (w *Workflow) Continue() error {
	_, err := os.Stat(w.statusPath)
	if err != nil {
		return err
	}
	err = w.Start()
	if err != nil {
		return err
	}
	return nil
}

// Percent returns the completion percentage between 0 and 100 of the workflow
func (w *Workflow) percent() float64 {
	current, total := w.progress()
	return float64(current) / float64(total) * 100
}

func (w *Workflow) progress() (int, int) {
	total := 0
	current := 0
	for i := range w.Status.Groups {
		if w.Status.Groups[i].Skip {
			continue
		}
		c, p := w.Status.Groups[i].progress()
		current += c
		total += p
	}
	return current, total
}

func (w *Workflow) writeStatus() error {
	w.Lock()
	defer w.Unlock()

	w.Status.Percent = int(w.percent())

	b, err := json.Marshal(&w.Status)
	if err != nil {
		slog.Error("unable to write status", "error", err)
	}

	err = os.WriteFile(w.statusPath, b, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (w *Workflow) writeSockets() error {
	for i := range w.ws {
		ws := w.ws[i]
		err := wsjson.Write(context.Background(), ws, w.Status)
		if err != nil {
			slog.Error("unable to write status to websocket.", "error", err)
			continue
		}
	}
	return nil
}
