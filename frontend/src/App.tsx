import { useRef, useState } from 'react'
import type { SubmitEvent } from 'react'
import './App.css'
import { streamResearch } from './api'
import { FileManager } from './FileManager'

function App() {
  const [requestText, setRequestText] = useState<string>('')
  const [markdown, setMarkdown] = useState<string>('')
  const [isResearching, setIsResearching] = useState<boolean>(false)
  const [researchError, setResearchError] = useState<string | null>(null)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())

  const abortControllerRef = useRef<AbortController | null>(null)
  const userAbortRequestedRef = useRef(false)

  const trimmedRequest = requestText.trim()

  async function handleResearchSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!trimmedRequest || isResearching) {
      return
    }

    const abortController = new AbortController()
    abortControllerRef.current = abortController
    userAbortRequestedRef.current = false
    setMarkdown('')
    setResearchError(null)
    setIsResearching(true)

    try {
      await streamResearch(
        trimmedRequest,
        (chunk) => {
          setMarkdown((current) => current + chunk)
        },
        { sourceIds: Array.from(selectedIds), signal: abortController.signal },
      )
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') {
        if (userAbortRequestedRef.current) {
          setResearchError('Research request cancelled.')
        }
      } else if (error instanceof Error) {
        setResearchError(error.message)
      } else {
        setResearchError('Research request failed.')
      }
    } finally {
      setIsResearching(false)
      abortControllerRef.current = null
      userAbortRequestedRef.current = false
    }
  }

  function handleAbort() {
    if (!abortControllerRef.current) {
      return
    }

    userAbortRequestedRef.current = true
    abortControllerRef.current.abort()
  }

  return (
    <main className="app-shell">
      <section className="hero" aria-labelledby="page-title">
        <h1 id="page-title">Research agent frontend</h1>
        <p>
          Manage your source files, pick which ones the agent should research over, and submit a
          request. The response panel displays streamed markdown as plain source text.
        </p>
      </section>

      <FileManager selectedIds={selectedIds} onSelectionChange={setSelectedIds} />

      <section className="panel" aria-labelledby="request-heading">
        <h2 id="request-heading">Research request</h2>
        <form className="form-stack" onSubmit={handleResearchSubmit}>
          <label htmlFor="research-request">What should the research agent investigate?</label>
          <textarea
            id="research-request"
            name="research-request"
            rows={6}
            value={requestText}
            onChange={(event) => setRequestText(event.currentTarget.value)}
            placeholder="Type here..."
          />
          <div className="button-row">
            <button type="submit" disabled={isResearching || !trimmedRequest}>
              {isResearching ? 'Researching...' : 'Start research'}
            </button>
            {isResearching ? (
              <button type="button" className="secondary" onClick={handleAbort}>
                Abort
              </button>
            ) : null}
          </div>
          {researchError ? <p className="error">{researchError}</p> : null}
        </form>
      </section>

      <section className="panel" aria-labelledby="response-heading">
        <h2 id="response-heading">Streaming response</h2>
        <pre className="markdown-output" aria-live="polite">
          {markdown || 'Submit a research request to see the markdown answer here.'}
        </pre>
      </section>
    </main>
  )
}

export default App
