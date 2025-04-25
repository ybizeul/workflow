package workflow

import "fmt"

// Errors definitions
var (
	WorkflowErrorNoGroups    = fmt.Errorf("no group definitions found")
	WorkflowErrorInvalidVars = fmt.Errorf("invalid variables definition")

	WorkflowErrorGroupMissingId    = fmt.Errorf("group missing id")
	WorkflowErrorGroupMissingTasks = fmt.Errorf("group missing tasks")

	WorkflowErrorTaskMissingId      = fmt.Errorf("task missing id")
	WorkflowErrorTaskMissingCommand = fmt.Errorf("task missing cmd")

	WorkflowErrorNotFinished = fmt.Errorf("workflow not finished")
)
