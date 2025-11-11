import React, { createContext, useContext, useReducer, ReactNode } from 'react'
import { NotebookState, Notebook, Session } from '../types/notebook'

interface NotebookContextType {
  state: NotebookState
  dispatch: React.Dispatch<NotebookAction>
}

type NotebookAction =
  | { type: 'SET_LOADING'; payload: boolean }
  | { type: 'SET_ERROR'; payload: string | null }
  | { type: 'SET_NOTEBOOKS'; payload: Notebook[] }
  | { type: 'SET_CURRENT_NOTEBOOK'; payload: Notebook | null }
  | { type: 'SET_ACTIVE_SESSIONS'; payload: Session[] }
  | { type: 'ADD_NOTEBOOK'; payload: Notebook }
  | { type: 'UPDATE_NOTEBOOK'; payload: Notebook }
  | { type: 'DELETE_NOTEBOOK'; payload: string }

const initialState: NotebookState = {
  notebooks: [],
  currentNotebook: null,
  activeSessions: [],
  isLoading: false,
  error: null,
}

const notebookReducer = (state: NotebookState, action: NotebookAction): NotebookState => {
  switch (action.type) {
    case 'SET_LOADING':
      return { ...state, isLoading: action.payload }
    case 'SET_ERROR':
      return { ...state, error: action.payload }
    case 'SET_NOTEBOOKS':
      return { ...state, notebooks: action.payload }
    case 'SET_CURRENT_NOTEBOOK':
      return { ...state, currentNotebook: action.payload }
    case 'SET_ACTIVE_SESSIONS':
      return { ...state, activeSessions: action.payload }
    case 'ADD_NOTEBOOK':
      return { ...state, notebooks: [...state.notebooks, action.payload] }
    case 'UPDATE_NOTEBOOK':
      return {
        ...state,
        notebooks: state.notebooks.map(nb => 
          nb.id === action.payload.id ? action.payload : nb
        ),
        currentNotebook: state.currentNotebook?.id === action.payload.id 
          ? action.payload 
          : state.currentNotebook
      }
    case 'DELETE_NOTEBOOK':
      return {
        ...state,
        notebooks: state.notebooks.filter(nb => nb.id !== action.payload),
        currentNotebook: state.currentNotebook?.id === action.payload 
          ? null 
          : state.currentNotebook
      }
    default:
      return state
  }
}

const NotebookContext = createContext<NotebookContextType | undefined>(undefined)

export const NotebookProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [state, dispatch] = useReducer(notebookReducer, initialState)

  return (
    <NotebookContext.Provider value={{ state, dispatch }}>
      {children}
    </NotebookContext.Provider>
  )
}

export const useNotebook = () => {
  const context = useContext(NotebookContext)
  if (context === undefined) {
    throw new Error('useNotebook must be used within a NotebookProvider')
  }
  return context
}