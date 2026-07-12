import { useEffect, useRef, useState } from 'react'
import type { ChangeEvent, SubmitEvent } from 'react'
import {
  deleteSource,
  getSource,
  listSources,
  updateSource,
  uploadSources,
  type Source,
} from './api'

type Props = {
  // Lifted selection state: which sources take part in the next research run.
  selectedIds: Set<string>
  onSelectionChange: (ids: Set<string>) => void
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export function FileManager({ selectedIds, onSelectionChange }: Props) {
  const [sources, setSources] = useState<Source[]>([])
  const [listError, setListError] = useState<string | null>(null)

  const [pendingFiles, setPendingFiles] = useState<FileList | null>(null)
  const [isUploading, setIsUploading] = useState(false)
  const [uploadError, setUploadError] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement | null>(null)

  // Viewing / editing the currently opened source.
  const [openId, setOpenId] = useState<string | null>(null)
  const [editContent, setEditContent] = useState<string>('')
  const [isLoadingContent, setIsLoadingContent] = useState(false)
  const [isSaving, setIsSaving] = useState(false)
  const [detailError, setDetailError] = useState<string | null>(null)

  async function refresh() {
    try {
      const list = await listSources()
      setSources(list)
      setListError(null)
      // Drop selections whose source no longer exists.
      const alive = new Set(list.map((s) => s.id))
      const next = new Set([...selectedIds].filter((id) => alive.has(id)))
      if (next.size !== selectedIds.size) onSelectionChange(next)
    } catch (error) {
      setListError(error instanceof Error ? error.message : 'Could not load sources.')
    }
  }

  useEffect(() => {
    void refresh()
    // Load the source list once on mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const files = event.currentTarget.files
    setPendingFiles(files && files.length > 0 ? files : null)
    setUploadError(null)
  }

  async function handleUpload(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!pendingFiles || pendingFiles.length === 0 || isUploading) return

    setIsUploading(true)
    setUploadError(null)
    try {
      await uploadSources(pendingFiles)
      setPendingFiles(null)
      if (fileInputRef.current) fileInputRef.current.value = ''
      await refresh()
    } catch (error) {
      setUploadError(error instanceof Error ? error.message : 'Upload failed.')
    } finally {
      setIsUploading(false)
    }
  }

  async function handleOpen(id: string) {
    if (openId === id) {
      setOpenId(null)
      return
    }
    setOpenId(id)
    setDetailError(null)
    setIsLoadingContent(true)
    try {
      const detail = await getSource(id)
      setEditContent(detail.content)
    } catch (error) {
      setDetailError(error instanceof Error ? error.message : 'Could not load content.')
    } finally {
      setIsLoadingContent(false)
    }
  }

  async function handleSave() {
    if (!openId || isSaving) return
    setIsSaving(true)
    setDetailError(null)
    try {
      await updateSource(openId, editContent)
      await refresh()
    } catch (error) {
      setDetailError(error instanceof Error ? error.message : 'Could not save.')
    } finally {
      setIsSaving(false)
    }
  }

  async function handleDelete(id: string) {
    setDetailError(null)
    try {
      await deleteSource(id)
      if (openId === id) setOpenId(null)
      await refresh()
    } catch (error) {
      setListError(error instanceof Error ? error.message : 'Could not delete source.')
    }
  }

  function toggleSelect(id: string) {
    const next = new Set(selectedIds)
    if (next.has(id)) next.delete(id)
    else next.add(id)
    onSelectionChange(next)
  }

  return (
    <section className="panel" aria-labelledby="files-heading">
      <h2 id="files-heading">Your sources</h2>

      <form className="form-stack" onSubmit={handleUpload}>
        <label htmlFor="source-files">Upload information sources</label>
        <input
          id="source-files"
          ref={fileInputRef}
          name="files"
          type="file"
          multiple
          onChange={handleFileChange}
          accept="text/*"
        />
        <div className="button-row">
          <button type="submit" disabled={isUploading || !pendingFiles}>
            {isUploading ? 'Uploading...' : 'Upload'}
          </button>
        </div>
        {uploadError ? <p className="error">{uploadError}</p> : null}
      </form>

      {listError ? <p className="error">{listError}</p> : null}

      {sources.length === 0 ? (
        <p className="empty-hint">No sources yet. Upload a file to get started.</p>
      ) : (
        <ul className="source-table" aria-label="Uploaded sources">
          {sources.map((source) => {
            const isOpen = openId === source.id
            const isSelected = selectedIds.has(source.id)
            return (
              <li key={source.id} className={`source-row${isSelected ? ' is-selected' : ''}`}>
                <div className="source-row-main">
                  <label className="source-select">
                    <input
                      type="checkbox"
                      checked={isSelected}
                      onChange={() => toggleSelect(source.id)}
                      aria-label={`Select ${source.name} for research`}
                    />
                  </label>
                  <div className="source-info">
                    <span className="source-name">{source.name}</span>
                    <span className="source-meta">{formatSize(source.size)}</span>
                  </div>
                  <div className="source-actions">
                    <button
                      type="button"
                      className="secondary"
                      onClick={() => void handleOpen(source.id)}
                    >
                      {isOpen ? 'Close' : 'View / Edit'}
                    </button>
                    <button
                      type="button"
                      className="secondary danger"
                      onClick={() => void handleDelete(source.id)}
                    >
                      Delete
                    </button>
                  </div>
                </div>

                {isOpen ? (
                  <div className="source-editor">
                    {isLoadingContent ? (
                      <p className="empty-hint">Loading content…</p>
                    ) : (
                      <>
                        <textarea
                          value={editContent}
                          onChange={(event) => setEditContent(event.currentTarget.value)}
                          rows={10}
                          aria-label={`Content of ${source.name}`}
                        />
                        <div className="button-row">
                          <button type="button" onClick={() => void handleSave()} disabled={isSaving}>
                            {isSaving ? 'Saving...' : 'Save'}
                          </button>
                        </div>
                      </>
                    )}
                    {detailError ? <p className="error">{detailError}</p> : null}
                  </div>
                ) : null}
              </li>
            )
          })}
        </ul>
      )}

      <p className="selection-hint">
        {selectedIds.size > 0
          ? `${selectedIds.size} source(s) selected for research.`
          : 'No sources selected — research will use all uploaded sources.'}
      </p>
    </section>
  )
}
