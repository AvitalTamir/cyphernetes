import { useState, useEffect } from 'react'
import { NotebookProvider } from './contexts/NotebookContext'
import { SettingsProvider } from './contexts/SettingsContext'
import { NotebookList } from './components/NotebookList'
import { NotebookEditor } from './components/NotebookEditor'
import { Header } from './components/Header'
import { Notebook } from './types/notebook'
import './App.css'

function App() {
  const [selectedNotebook, setSelectedNotebook] = useState<Notebook | null>(null)
  const [notebooks, setNotebooks] = useState<Notebook[]>([])

  useEffect(() => {
    // Load notebooks from API
    loadNotebooks()
  }, [])

  const loadNotebooks = async () => {
    try {
      const response = await fetch('/api/notebooks')
      const data = await response.json()
      setNotebooks(data)
    } catch (error) {
      console.error('Failed to load notebooks:', error)
    }
  }

  const handleNotebookSelect = (notebook: Notebook) => {
    setSelectedNotebook(notebook)
  }

  const handleNotebookCreate = async (name: string) => {
    try {
      const response = await fetch('/api/notebooks', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ name }),
      })
      const newNotebook = await response.json()
      setNotebooks([...notebooks, newNotebook])
      setSelectedNotebook(newNotebook)
    } catch (error) {
      console.error('Failed to create notebook:', error)
    }
  }

  const handleNotebookDelete = async (notebookId: string) => {
    try {
      const response = await fetch(`/api/notebooks/${notebookId}`, {
        method: 'DELETE',
      })
      if (response.ok) {
        // Remove the deleted notebook from the local state
        setNotebooks(notebooks.filter(nb => nb.id !== notebookId))
        // If the deleted notebook was selected, clear the selection
        if (selectedNotebook?.id === notebookId) {
          setSelectedNotebook(null)
        }
      } else {
        const error = await response.json()
        console.error('Failed to delete notebook:', error.error)
        alert('Failed to delete notebook: ' + error.error)
      }
    } catch (error) {
      console.error('Failed to delete notebook:', error)
      alert('Failed to delete notebook')
    }
  }

  const handleNotebookUpdate = async (notebookId: string, name: string) => {
    try {
      const response = await fetch(`/api/notebooks/${notebookId}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ name }),
      })
      if (response.ok) {
        const updatedNotebook = await response.json()
        // Update the notebook in local state
        setNotebooks(notebooks.map(nb => nb.id === notebookId ? updatedNotebook : nb))
        // Update selected notebook if it's the one being updated
        if (selectedNotebook?.id === notebookId) {
          setSelectedNotebook(updatedNotebook)
        }
        return true
      } else {
        const error = await response.json()
        console.error('Failed to update notebook:', error.error)
        alert('Failed to update notebook: ' + error.error)
        return false
      }
    } catch (error) {
      console.error('Failed to update notebook:', error)
      alert('Failed to update notebook')
      return false
    }
  }

  const handleBack = () => {
    setSelectedNotebook(null)
    // Refresh the notebooks list to get updated cell counts
    loadNotebooks()
  }

  return (
    <SettingsProvider>
      <NotebookProvider>
        <div className="app">
          <Header />
          <main className="main-content">
            {selectedNotebook ? (
              <NotebookEditor
                notebook={selectedNotebook}
                onBack={handleBack}
                onUpdate={handleNotebookUpdate}
              />
            ) : (
              <NotebookList
                notebooks={notebooks}
                onSelect={handleNotebookSelect}
                onCreate={handleNotebookCreate}
                onDelete={handleNotebookDelete}
              />
            )}
          </main>
        </div>
      </NotebookProvider>
    </SettingsProvider>
  )
}

export default App