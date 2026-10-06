import React from 'react'
import ReactDOM from 'react-dom/client'
import { App } from './App'
import { paintFirstFrame } from './colorScheme'
import './fonts.css'
import './styles.css'

paintFirstFrame()

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
