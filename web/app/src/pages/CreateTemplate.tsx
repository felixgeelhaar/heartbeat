import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import type { CreateTemplateMetric, CreateTemplatePayload } from '../types'

interface MetricFormState {
  name: string
  description_good: string
  description_bad: string
}

const SPOTIFY_METRICS: MetricFormState[] = [
  { name: 'Easy to Release', description_good: 'Releasing is simple, safe, painless & mostly automated', description_bad: 'Releasing is risky, painful, lots of manual work, and takes forever' },
  { name: 'Suitable Process', description_good: 'Our way of working fits us perfectly', description_bad: 'Our way of working sucks' },
  { name: 'Tech Quality', description_good: "We're proud of the quality of our code! It is clean, easy to read, and has great test coverage", description_bad: 'Our code is a pile of dung, and technical debt is raging out of control' },
  { name: 'Value', description_good: "We deliver great stuff! We're proud of it and our stakeholders are really happy", description_bad: 'We deliver crap. We feel ashamed to deliver it. Our stakeholders hate us' },
  { name: 'Speed', description_good: 'We get stuff done really quickly. No waiting, no delays', description_bad: 'We never seem to get done with anything. Stories keep getting stuck on dependencies' },
  { name: 'Mission', description_good: 'We know exactly why we are here, and we are really excited about it', description_bad: 'We have no idea why we are here, there is no clear mission' },
  { name: 'Fun', description_good: 'We love going to work, and have great fun working together', description_bad: 'Boooooring' },
  { name: 'Learning', description_good: "We're learning lots of interesting stuff all the time!", description_bad: 'We never have time to learn anything' },
  { name: 'Support', description_good: 'We always get great support & help when we ask for it!', description_bad: "We keep getting stuck because we can't get the support & help we ask for" },
  { name: 'Pawns or Players', description_good: 'We are in control of our destiny! We decide what to build and how to build it', description_bad: 'We are just pawns in a game of chess, with no influence over what we build or how we build it' },
]

const emptyMetric = (): MetricFormState => ({
  name: '',
  description_good: '',
  description_bad: '',
})

export function CreateTemplate() {
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [metrics, setMetrics] = useState<MetricFormState[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const addMetric = () => {
    setMetrics([...metrics, emptyMetric()])
  }

  const addSpotifyMetric = (sm: MetricFormState) => {
    if (metrics.some(m => m.name === sm.name)) return
    setMetrics([...metrics, { ...sm }])
  }

  const addAllSpotifyMetrics = () => {
    const existing = new Set(metrics.map(m => m.name))
    const toAdd = SPOTIFY_METRICS.filter(sm => !existing.has(sm.name))
    setMetrics([...metrics, ...toAdd.map(sm => ({ ...sm }))])
  }

  const removeMetric = (index: number) => {
    setMetrics(metrics.filter((_, i) => i !== index))
  }

  const updateMetric = (index: number, field: keyof MetricFormState, value: string) => {
    setMetrics(metrics.map((m, i) => i === index ? { ...m, [field]: value } : m))
  }

  const isSpotifyMetricAdded = (sm: MetricFormState) => metrics.some(m => m.name === sm.name)

  const isValid = () => {
    if (!name.trim()) return false
    if (metrics.length === 0) return false
    return metrics.every(m => m.name.trim() && m.description_good.trim() && m.description_bad.trim())
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!isValid()) return

    setSubmitting(true)
    setError(null)

    const payload: CreateTemplatePayload = {
      name: name.trim(),
      description: description.trim(),
      metrics: metrics.map((m): CreateTemplateMetric => ({
        name: m.name.trim(),
        description_good: m.description_good.trim(),
        description_bad: m.description_bad.trim(),
      })),
    }

    try {
      const res = await fetch('/api/templates', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })

      if (res.ok) {
        navigate('/')
      } else {
        const text = await res.text()
        setError(text || 'Failed to create template')
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Network error')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div>
      <Link to="/" className="back-link">
        &#8592; Back to dashboard
      </Link>

      <div className="page-header">
        <h1>Create Template</h1>
        <p style={{ color: 'var(--text-secondary)', fontSize: '14px', marginTop: '4px' }}>
          Define the metrics your team will be evaluated on.
        </p>
      </div>

      <form onSubmit={handleSubmit}>
        <div className="glass-card" style={{ marginBottom: '24px' }}>
          <div className="form-group">
            <label className="form-label">Template Name</label>
            <input
              type="text"
              className="form-input"
              placeholder="e.g., Sprint Health Check"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>

          <div className="form-group" style={{ marginBottom: 0 }}>
            <label className="form-label">Description</label>
            <textarea
              className="form-textarea"
              placeholder="What is this template for?"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={2}
            />
          </div>
        </div>

        {/* Spotify Quick-Add */}
        <div className="glass-card" style={{ marginBottom: '24px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
            <div>
              <div style={{ fontSize: '14px', fontWeight: 600, color: 'var(--text-primary)' }}>
                Spotify Health Check Metrics
              </div>
              <div style={{ fontSize: '12px', color: 'var(--text-secondary)', marginTop: '2px' }}>
                Click to add individual metrics or add all at once
              </div>
            </div>
            <button
              type="button"
              className="btn btn-secondary btn-sm"
              onClick={addAllSpotifyMetrics}
              style={{ whiteSpace: 'nowrap' }}
            >
              Add All
            </button>
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
            {SPOTIFY_METRICS.map((sm) => {
              const added = isSpotifyMetricAdded(sm)
              return (
                <button
                  key={sm.name}
                  type="button"
                  onClick={() => !added && addSpotifyMetric(sm)}
                  style={{
                    padding: '6px 14px',
                    borderRadius: '20px',
                    border: added ? '1px solid var(--green)' : '1px solid rgba(255,255,255,0.12)',
                    background: added ? 'rgba(34,197,94,0.15)' : 'rgba(255,255,255,0.05)',
                    color: added ? 'var(--green)' : 'var(--text-primary)',
                    fontSize: '13px',
                    cursor: added ? 'default' : 'pointer',
                    transition: 'all 0.15s ease',
                    opacity: added ? 0.7 : 1,
                  }}
                >
                  {added ? '✓ ' : '+ '}{sm.name}
                </button>
              )
            })}
          </div>
        </div>

        <div className="section-title">Metrics ({metrics.length})</div>

        {metrics.length === 0 && (
          <div className="glass-card" style={{ textAlign: 'center', padding: '32px', color: 'var(--text-secondary)', marginBottom: '16px' }}>
            No metrics added yet. Pick from Spotify metrics above or add your own below.
          </div>
        )}

        {metrics.map((metric, index) => (
          <div key={index} className="metric-form-item">
            <div className="metric-form-header">
              <span className="metric-form-number">Metric {index + 1}</span>
              <button
                type="button"
                className="btn btn-danger btn-sm"
                onClick={() => removeMetric(index)}
              >
                Remove
              </button>
            </div>

            <div className="form-group">
              <label className="form-label">Metric Name</label>
              <input
                type="text"
                className="form-input"
                placeholder="e.g., Team Collaboration"
                value={metric.name}
                onChange={(e) => updateMetric(index, 'name', e.target.value)}
              />
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
              <div className="form-group" style={{ marginBottom: 0 }}>
                <label className="form-label" style={{ color: 'var(--green)' }}>
                  What Good Looks Like
                </label>
                <textarea
                  className="form-textarea"
                  placeholder="Describe the ideal state..."
                  value={metric.description_good}
                  onChange={(e) => updateMetric(index, 'description_good', e.target.value)}
                  rows={2}
                />
              </div>
              <div className="form-group" style={{ marginBottom: 0 }}>
                <label className="form-label" style={{ color: 'var(--red)' }}>
                  What Bad Looks Like
                </label>
                <textarea
                  className="form-textarea"
                  placeholder="Describe problematic state..."
                  value={metric.description_bad}
                  onChange={(e) => updateMetric(index, 'description_bad', e.target.value)}
                  rows={2}
                />
              </div>
            </div>
          </div>
        ))}

        <button
          type="button"
          className="btn btn-secondary"
          onClick={addMetric}
          style={{ marginTop: '4px', marginBottom: '24px' }}
        >
          + Add Custom Metric
        </button>

        {error && (
          <div style={{
            marginBottom: '16px',
            padding: '12px 16px',
            background: 'var(--red-dim)',
            border: '1px solid rgba(239, 68, 68, 0.2)',
            borderRadius: 'var(--radius-md)',
            color: 'var(--red)',
            fontSize: '14px',
          }}>
            {error}
          </div>
        )}

        <button
          type="submit"
          className="btn btn-primary btn-lg"
          disabled={!isValid() || submitting}
        >
          {submitting ? 'Creating...' : `Create Template (${metrics.length} metrics)`}
        </button>
      </form>
    </div>
  )
}
