export interface WorkflowStatus {
    started: boolean,
    finished: boolean,
    percent: number,
    lastMessage: string,
    currentGroup: string,
    currentTask: string,
    error: string,
    groups: Group[],
    osrelease: string,
}

export class Group {
    id: string
    percent: number
    started: boolean
    finished: boolean
    skip: boolean
    lastMessage: string
    error: string|undefined
    tasks: Task[]
    constructor(values: {id:string, percent:number, started: boolean, finished:boolean, skip: boolean, lastMessage: string}) {
        this.id = values.id
        this.percent = values.percent
        this.started = values.started
        this.finished = values.finished
        this.skip = values.skip
        this.lastMessage = values.lastMessage
        this.error = undefined
        this.tasks = []
    }
}

export interface Task {
    id: string,
    weight: number,
    started: boolean,
    finished: boolean,
    percent: number,
}

export class WorkflowClient {
    endpoint: string
    websocket: WebSocket|undefined

    status: WorkflowStatus|undefined

    onstatus?: (status: WorkflowStatus) => void
    ongroup?: Record<string,(group: Group) => void>
    onerror?: (status: WorkflowStatus) => void

    constructor(endpoint: string) {
        this.endpoint = endpoint
    }

    read() {
        if (this.websocket) {
            return
        }
        this.websocket = new WebSocket(this.endpoint)

        if (this.websocket === undefined) {
            console.error("WebSocket not ready")
            return
        }

        this.websocket.onmessage = (e) => {
            const status = JSON.parse(e.data)

            if (status) {
                if (this.onstatus) {
                    this.onstatus(status);
                }

                for (const group of status.groups) {
                    if (this.ongroup && this.ongroup[group.id]) {
                        this.ongroup[group.id](group)
                    }
                }

                if (status.error) {
                    if (this.onerror) {
                        this.onerror(status);
                    }
                }
            }
            this.status = status
        }

        this.websocket.onerror = (e) => {
            console.log(e)
        }

        this.websocket.onclose = (e) => {
            this.websocket = undefined
            setTimeout(() => {
                console.log("WebSocket closed, trying to reconnect", e)
                this.read();
              }, 1000);
        }
        this.websocket.onopen = (e) => {
            console.log("connected to websocket",e)
        }
    }
    
    onStatus(callback: (status: WorkflowStatus) => void) {
        this.onstatus = callback
    }

    onGroup(id: string, callback: (group: Group) => void) {
        if (this.ongroup === undefined) {
            this.ongroup = {}
        }
        this.ongroup[id] = callback
    }

    onError(callback: (status: WorkflowStatus) => void) {
        this.onerror = callback
    }
}