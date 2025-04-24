import { useEffect, useState } from 'react'
import './App.css'
import { WorkflowClient, WorkflowStatus } from './Workflow'

function App() {
  const [status, setStatus] = useState<WorkflowStatus|undefined>(undefined)

  useEffect(() => {
    const endpoint = 'ws://localhost:5174/wf'
    const wf = new WorkflowClient(endpoint)
    wf.onStatus((s) => {
      setStatus(s)
    })
    wf.read()
  },[])

  return (
    <>
      Output:
      <pre>
        Last message: {status?.lastMessage}
      </pre>
    </>
  )
}

export default App
