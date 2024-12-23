import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'

console.log(`
 ______ _____  / /  ___ _______  ___ / /____ ___
/ __/ // / _ \\/ _ \\/ -_) __/ _ \\/ -_) __/ -_|_-<
\\__/\\_, / .__/_//_/\\__/_/ /_//_/\\__/\\__/\\__/___/
   /___/_/ Web Interface

Wanna help build this thing?
Visit https://github.com/AvitalTamir/cyphernetes`)
ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)