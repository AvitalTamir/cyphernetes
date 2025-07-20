import React, { useState } from 'react'
import { X, Palette, Monitor } from 'lucide-react'
import { useSettings } from '../contexts/SettingsContext'
import './SettingsModal.css'

interface SettingsModalProps {
  isOpen: boolean
  onClose: () => void
}

export const SettingsModal: React.FC<SettingsModalProps> = ({ isOpen, onClose }) => {
  const { theme, themeName, setTheme, availableThemes } = useSettings()

  if (!isOpen) return null

  const handleThemeChange = (newThemeName: string) => {
    setTheme(newThemeName)
  }

  return (
    <div className="settings-modal-overlay" onClick={onClose}>
      <div className="settings-modal" onClick={e => e.stopPropagation()}>
        <div className="settings-modal-header">
          <h2>Settings</h2>
          <button className="settings-modal-close" onClick={onClose}>
            <X size={20} />
          </button>
        </div>
        
        <div className="settings-modal-content">
          <div className="settings-section">
            <div className="settings-section-header">
              <Palette size={18} />
              <h3>Theme</h3>
            </div>
            
            <div className="theme-grid">
              {availableThemes.map((themeOption) => (
                <div
                  key={themeOption.name}
                  className={`theme-card ${themeName === themeOption.name ? 'active' : ''}`}
                  onClick={() => handleThemeChange(themeOption.name)}
                >
                  <div className="theme-preview">
                    <div 
                      className="theme-color-bar"
                      style={{ 
                        background: `linear-gradient(135deg, ${themeOption.colors.primary}, ${themeOption.colors.secondary})` 
                      }}
                    />
                    <div className="theme-colors">
                      <div 
                        className="theme-color-dot"
                        style={{ backgroundColor: themeOption.colors.background }}
                      />
                      <div 
                        className="theme-color-dot"
                        style={{ backgroundColor: themeOption.colors.surface }}
                      />
                      <div 
                        className="theme-color-dot"
                        style={{ backgroundColor: themeOption.colors.primary }}
                      />
                      <div 
                        className="theme-color-dot"
                        style={{ backgroundColor: themeOption.colors.accent }}
                      />
                    </div>
                  </div>
                  <div className="theme-name">
                    {themeOption.displayName}
                  </div>
                  {themeName === themeOption.name && (
                    <div className="theme-active-indicator">
                      <Monitor size={14} />
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>
        </div>
        
        <div className="settings-modal-footer">
          <button className="btn btn-secondary" onClick={onClose}>
            Close
          </button>
        </div>
      </div>
    </div>
  )
}