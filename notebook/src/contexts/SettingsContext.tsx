import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react'

export interface Theme {
  name: string
  displayName: string
  colors: {
    background: string
    surface: string
    primary: string
    secondary: string
    text: string
    textSecondary: string
    border: string
    accent: string
    success: string
    warning: string
    error: string
  }
}

export const themes: Record<string, Theme> = {
  light: {
    name: 'light',
    displayName: 'Light',
    colors: {
      background: '#ffffff',
      surface: '#f8f9fa',
      primary: '#6c5ce7',
      secondary: '#a29bfe',
      text: '#2d3436',
      textSecondary: '#636e72',
      border: '#ddd',
      accent: '#0984e3',
      success: '#00b894',
      warning: '#fdcb6e',
      error: '#e84393'
    }
  },
  dark: {
    name: 'dark',
    displayName: 'Dark',
    colors: {
      background: '#1a1a1a',
      surface: '#2d2d2d',
      primary: '#a29bfe',
      secondary: '#6c5ce7',
      text: '#f8f9fa',
      textSecondary: '#b2bec3',
      border: '#444',
      accent: '#74b9ff',
      success: '#00b894',
      warning: '#fdcb6e',
      error: '#fd79a8'
    }
  },
  ocean: {
    name: 'ocean',
    displayName: 'Ocean',
    colors: {
      background: '#0c1821',
      surface: '#1b2838',
      primary: '#3282b8',
      secondary: '#0f3460',
      text: '#bbe1fa',
      textSecondary: '#7fb3d5',
      border: '#2c5282',
      accent: '#00d2d3',
      success: '#48bb78',
      warning: '#ed8936',
      error: '#f56565'
    }
  },
  forest: {
    name: 'forest',
    displayName: 'Forest',
    colors: {
      background: '#1a2f1a',
      surface: '#2d4a2d',
      primary: '#68d391',
      secondary: '#48bb78',
      text: '#f0fff4',
      textSecondary: '#c6f6d5',
      border: '#38a169',
      accent: '#4fd1c7',
      success: '#68d391',
      warning: '#fbb040',
      error: '#fc8181'
    }
  },
  sunset: {
    name: 'sunset',
    displayName: 'Sunset',
    colors: {
      background: '#2d1b1b',
      surface: '#4a2d2d',
      primary: '#ff7675',
      secondary: '#fd79a8',
      text: '#fff5f5',
      textSecondary: '#fed7d7',
      border: '#e53e3e',
      accent: '#ffa726',
      success: '#66bb6a',
      warning: '#ffb74d',
      error: '#ef5350'
    }
  },
  desert: {
    name: 'desert',
    displayName: 'Desert',
    colors: {
      background: '#2a1810',
      surface: '#3d2919',
      primary: '#d4930b',
      secondary: '#e67e22',
      text: '#f5e6d3',
      textSecondary: '#d2a679',
      border: '#8b5a2b',
      accent: '#c0783b',
      success: '#76c893',
      warning: '#ffd23f',
      error: '#ff6b6b'
    }
  }
}

interface SettingsContextType {
  theme: Theme
  themeName: string
  setTheme: (themeName: string) => void
  availableThemes: Theme[]
}

const SettingsContext = createContext<SettingsContextType | undefined>(undefined)

export const useSettings = () => {
  const context = useContext(SettingsContext)
  if (!context) {
    throw new Error('useSettings must be used within a SettingsProvider')
  }
  return context
}

interface SettingsProviderProps {
  children: ReactNode
}

export const SettingsProvider: React.FC<SettingsProviderProps> = ({ children }) => {
  const [themeName, setThemeName] = useState<string>('light')

  // Load theme from localStorage on mount
  useEffect(() => {
    const savedTheme = localStorage.getItem('cyphernetes-theme')
    if (savedTheme && themes[savedTheme]) {
      setThemeName(savedTheme)
    }
  }, [])

  // Apply theme to CSS variables
  useEffect(() => {
    const theme = themes[themeName]
    if (theme) {
      const root = document.documentElement
      Object.entries(theme.colors).forEach(([key, value]) => {
        root.style.setProperty(`--color-${key}`, value)
      })
    }
  }, [themeName])

  const setTheme = (newThemeName: string) => {
    if (themes[newThemeName]) {
      setThemeName(newThemeName)
      localStorage.setItem('cyphernetes-theme', newThemeName)
    }
  }

  const value: SettingsContextType = {
    theme: themes[themeName],
    themeName,
    setTheme,
    availableThemes: Object.values(themes)
  }

  return (
    <SettingsContext.Provider value={value}>
      {children}
    </SettingsContext.Provider>
  )
}