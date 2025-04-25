import { useEffect, useState } from 'react'
import './App.css'
import { WorkflowClient, WorkflowStatus } from './Workflow'

function App() {
  const [status, setStatus] = useState<WorkflowStatus|undefined>(undefined)

  const start = () => {
    fetch('/go/start', {
      method: 'POST',
    }).then((response) => {
      if (response.ok) {
        console.log('Workflow started')
      } else {
        console.error('Error starting workflow')
      }
    }).catch((error) => {
      console.error('Error starting workflow', error)
    })
  }

  const stop = () => {
    fetch('/go/stop', {
      method: 'POST',
    }).then((response) => {
      if (response.ok) {
        console.log('Workflow stopped')
      } else {
        console.error('Error stopping workflow')
      }
    }).catch((error) => {
      console.error('Error stopping workflow', error)
    })
  }


  useEffect(() => {
    const endpoint = 'ws://localhost:5174/go/wf'
    const wf = new WorkflowClient(endpoint)
    wf.onStatus((s) => {
      console.log(s)
      setStatus(s)
    })
    wf.read()
  },[])

  return (
    <>
      <button disabled={!(!status?.started || status?.finished)} onClick={() => {start()}}>Start</button>
      <button disabled={!(status?.started && !status?.finished)} onClick={() => {stop()}}>Stop</button><br/>
      Output:
      <pre>
        Last message: {status?.lastMessage}
      </pre>
    </>
  )
}

export default App
