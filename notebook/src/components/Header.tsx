import React, { useState } from 'react'
import { UserPlus, Settings } from 'lucide-react'
import { SettingsModal } from './SettingsModal'
import { ShareModal } from './ShareModal'
import { useNotebook } from '../contexts/NotebookContext'
import './Header.css'

interface HeaderProps {
  isSharedMode?: boolean
}

export const Header: React.FC<HeaderProps> = ({ isSharedMode = false }) => {
  const [isSettingsOpen, setIsSettingsOpen] = useState(false)
  const [isShareOpen, setIsShareOpen] = useState(false)
  const { state } = useNotebook()

  return (
    <>
      <header className="header">
        <div className="header-content">
          <div className="header-left">
            <h1 className="header-title">
              <img src="/logo.png" alt="Cyphernetes" className="header-logo" />
              Cyphernetes Notebooks
            </h1>
          </div>
          <div className="header-right">
            {!isSharedMode && (
              <button 
                className="header-button" 
                title={state.currentNotebook ? "Share Notebook" : "Select a notebook to share"}
                disabled={!state.currentNotebook}
                onClick={() => setIsShareOpen(true)}
              >
                <UserPlus size={16} />
              </button>
            )}
            <button 
              className="header-button" 
              title="Settings"
              onClick={() => setIsSettingsOpen(true)}
            >
              <Settings size={16} />
            </button>
          </div>
        </div>
      </header>
      
      <SettingsModal 
        isOpen={isSettingsOpen} 
        onClose={() => setIsSettingsOpen(false)} 
      />
      
      {!isSharedMode && (
        <ShareModal
          isOpen={isShareOpen}
          onClose={() => setIsShareOpen(false)}
          notebookId={state.currentNotebook?.id}
        />
      )}
    </>
  )
}