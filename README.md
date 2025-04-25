## Introduction

Workflow provides a simple engine to sequentially run groups of shell
tasks, while providing a websocket to monitor progress.

Workflows are defined using a yaml file which contains variables declaration
and groups of tasks.

Tasks shell scripts can use functions like `output` and `progress` to publish
meaningful information to the websocket.

Tasks can have an optional relative `weight` value so a progress expressed in
percent is added in the status message.

## Working with websockets

[examples/workflow-react](examples/workflow-react) shows how to use workflow
with a react frontend

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
