import React, { useState, useEffect } from 'react'
import { X, Copy, Check, Clock, Globe } from 'lucide-react'
import './ShareModal.css'

interface ShareModalProps {
  isOpen: boolean
  onClose: () => void
  notebookId?: string
}

interface ShareResponse {
  share_url: string
  expires_at: string
  expires_in: number
}

export const ShareModal: React.FC<ShareModalProps> = ({
  isOpen,
  onClose,
  notebookId
}) => {
  const [shareUrl, setShareUrl] = useState<string>('')
  const [expiresIn, setExpiresIn] = useState<number>(0)
  const [isLoading, setIsLoading] = useState(false)
  const [copied, setCopied] = useState(false)
  const [error, setError] = useState<string>('')

  // Generate share link
  const generateShareLink = async () => {
    if (!notebookId) {
      setError('No notebook selected')
      return
    }

    setIsLoading(true)
    setError('')

    try {
      const response = await fetch('/api/share/generate-token', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          notebook_id: notebookId
        })
      })

      if (!response.ok) {
        throw new Error('Failed to generate share link')
      }

      const data: ShareResponse = await response.json()
      setShareUrl(data.share_url)
      setExpiresIn(data.expires_in)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to generate share link')
    } finally {
      setIsLoading(false)
    }
  }

  // Copy to clipboard
  const copyToClipboard = async () => {
    try {
      await navigator.clipboard.writeText(shareUrl)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch (err) {
      console.error('Failed to copy:', err)
    }
  }

  // Countdown timer
  useEffect(() => {
    if (expiresIn <= 0) return

    const interval = setInterval(() => {
      setExpiresIn(prev => {
        if (prev <= 1) {
          setShareUrl('')
          return 0
        }
        return prev - 1
      })
    }, 1000)

    return () => clearInterval(interval)
  }, [expiresIn])

  // Reset state when modal opens/closes
  useEffect(() => {
    if (isOpen && !shareUrl) {
      generateShareLink()
    }
    if (!isOpen) {
      setShareUrl('')
      setExpiresIn(0)
      setError('')
      setCopied(false)
    }
  }, [isOpen])

  // Handle ESC key
  useEffect(() => {
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && isOpen) {
        onClose()
      }
    }

    if (isOpen) {
      document.addEventListener('keydown', handleEscape)
      return () => document.removeEventListener('keydown', handleEscape)
    }
  }, [isOpen, onClose])

  // Handle click outside modal
  const handleOverlayClick = (event: React.MouseEvent<HTMLDivElement>) => {
    if (event.target === event.currentTarget) {
      onClose()
    }
  }

  // Format time remaining
  const formatTimeRemaining = (seconds: number): string => {
    const minutes = Math.floor(seconds / 60)
    const remainingSeconds = seconds % 60
    return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`
  }

  if (!isOpen) return null

  return (
    <div className="share-modal-overlay" onClick={handleOverlayClick}>
      <div className="share-modal">
        <div className="share-modal-header">
          <h2>Share Notebook</h2>
          <button className="share-modal-close" onClick={onClose}>
            <X size={20} />
          </button>
        </div>

        <div className="share-modal-content">
          {isLoading && (
            <div className="share-modal-loading">
              <div className="loading-spinner"></div>
              <p>Generating secure share link...</p>
            </div>
          )}

          {error && (
            <div className="share-modal-error">
              <p>{error}</p>
              <button onClick={generateShareLink} className="retry-button">
                Try Again
              </button>
            </div>
          )}

          {shareUrl && (
            <div className="share-modal-success">
              <div className="share-info">
                <Globe size={20} className="share-icon" />
                <div className="share-details">
                  <p className="share-description">
                    Anyone with this link can view your notebook
                  </p>
                  <div className="share-expiry">
                    <Clock size={16} />
                    <span>Expires in {formatTimeRemaining(expiresIn)}</span>
                  </div>
                </div>
              </div>

              <div className="share-url-container">
                <input
                  type="text"
                  value={shareUrl}
                  readOnly
                  className="share-url-input"
                  onClick={(e) => (e.target as HTMLInputElement).select()}
                />
                <button 
                  onClick={copyToClipboard}
                  className={`copy-button ${copied ? 'copied' : ''}`}
                >
                  {copied ? <Check size={16} /> : <Copy size={16} />}
                  {copied ? 'Copied!' : 'Copy'}
                </button>
              </div>

              <div className="share-modal-footer">
                <p className="security-note">
                  🔐 This link is secured with a unique token and will expire automatically
                </p>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}