## Introduction

Workflow provides a simple engine to sequentially run groups of shell
tasks, while providing a websocket to monitor progress.

Workflows are defined using a yaml file which contains variables declaration
and groups of tasks.

Tasks shell scripts can use functions like `output` and `progress` to publish
meaningful information to the websocket.

Tasks can have an optional relative `weight` value so a progress expressed in
percent is added in the status message.

Full API documentation on [pkg.go.dev](https://pkg.go.dev/github.com/ybizeul/workflow)

## Working with websockets

[example/workflow-react](example/workflow-react) shows how to use workflow
with a react frontend

## Sending feedback during task execution

Shell scripts can use special shell functions to provide output and progress
information to the workflow. The functions are:
- `output`: will send a message to the workflow. It will be
available in `LastMessage` for the task, group, and workflow.
- `progress`: will send a number between 0 and 1 to indicate the relative
progress of the current task.
- `error`: will send an error description if something unexpected happens,
and will be available in `Error` field of task, group and workflow.

## Example

This workflow declares a variable `OS` with the output of `uname` command, then
it defines two groups of tasks, one for Linux and one for Darwin.

Groups are being skipped based on the content of `OS`

    vars:
      OS: uname
    groups:
      - id: linuxGroup
        skip_cmd: |
          [ "$OS" != "Linux" ]
        tasks:
          - id: task1
            weight: 50
            cmd: |
              output `ip a s eth0|grep inet`
      - id: darwinGroup
        skip_cmd: |
          [ "$OS" != "Darwin" ]
        tasks:
          - id: task1
            weight: 50
            cmd: |
              output `ifconfig en0|grep "inet "`
