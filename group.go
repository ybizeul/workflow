package workflow

// Group represents a group of tasks in a workflow. It will give informations
// about the execution state like completion percentage, any error that occurred
// and the last message received from the tasks.
// Groups can be skipped if the command in skip_cmd from the yaml definition
// returns a zero status code.
type Group struct {
	Id          string  `json:"id"`
	Tasks       []*Task `json:"tasks"`
	skip_cmd    string
	Skip        bool    `json:"skip"`
	Percent     float64 `json:"percent"`
	Started     bool    `json:"started"`
	Finished    bool    `json:"finished"`
	LastMessage string  `json:"lastMessage"`
	Error       string  `json:"error"`
}

func newGroup(y map[string]any) (*Group, error) {
	id, ok := y["id"].(string)
	if !ok {
		return nil, WorkflowErrorGroupMissingId
	}

	skip_cmd, _ := y["skip_cmd"].(string)

	tasks, ok := y["tasks"].([]any)
	if !ok {
		return nil, WorkflowErrorGroupMissingTasks
	}

	result := &Group{
		Id:       id,
		Tasks:    []*Task{},
		skip_cmd: skip_cmd,
	}

	for i := range tasks {
		task, err := newTask(tasks[i].(map[string]any))
		if err != nil {
			return nil, err
		}
		result.Tasks = append(result.Tasks, task)
	}

	return result, nil
}

func (w *Group) progress() (float64, float64) {
	// Calculate total weight
	total := 0.0
	current := 0.0
	finished := true
	for i := range w.Tasks {
		task := w.Tasks[i]
		if task.Finished {
			current += float64(task.Weight)
		} else {
			finished = false
			current += float64(task.Weight) * task.Percent
		}
		total += float64(task.Weight)
	}

	w.Finished = finished
	if total > 0 {
		w.Percent = float64(current) / float64(total) * 100
	}
	return current, total
}
