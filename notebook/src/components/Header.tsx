import React from 'react'
import './Header.css'

export const Header: React.FC = () => {
  return (
    <header className="header">
      <div className="header-content">
        <div className="header-left">
          <h1 className="header-title">
            🚀 Cyphernetes Notebooks
          </h1>
        </div>
        <div className="header-right">
          <button className="header-button">
            Share
          </button>
          <button className="header-button">
            Settings
          </button>
        </div>
      </div>
    </header>
  )
}