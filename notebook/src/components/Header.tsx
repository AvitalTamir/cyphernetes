import React from 'react'
import { UserPlus, Settings } from 'lucide-react'
import './Header.css'

export const Header: React.FC = () => {
  return (
    <header className="header">
      <div className="header-content">
        <div className="header-left">
          <h1 className="header-title">
            <img src="/logo.png" alt="Cyphernetes" className="header-logo" />
            Cyphernetes Notebooks
          </h1>
        </div>
        <div className="header-right">
          <button className="header-button" title="Share">
            <UserPlus size={16} />
          </button>
          <button className="header-button" title="Settings">
            <Settings size={16} />
          </button>
        </div>
      </div>
    </header>
  )
}