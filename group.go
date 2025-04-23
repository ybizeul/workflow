package workflow

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

type SkipCmd string

func NewGroup(y map[string]any) (*Group, error) {
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
		task, err := NewTask(tasks[i].(map[string]any))
		if err != nil {
			return nil, err
		}
		result.Tasks = append(result.Tasks, task)
	}

	return result, nil
}

func (w *Group) Progress() (int, int) {
	// Calculate total weight
	total := 0
	current := 0
	finished := true
	for i := range w.Tasks {
		task := w.Tasks[i]
		if task.Finished {
			current += task.Weight
		} else {
			finished = false
			current += int(float64(task.Weight) * task.Percent)
		}
		total += task.Weight
	}

	w.Finished = finished
	if total > 0 {
		w.Percent = float64(current) / float64(total) * 100
	}
	return current, total
}
