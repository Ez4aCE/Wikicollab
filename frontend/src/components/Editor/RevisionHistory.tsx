import React, { useState } from 'react';
import { apiFetch } from '../../api/client';

interface Revision {
  id: string;
  version: number;
  created_by: string;
  created_at: string;
}

interface RevisionHistoryProps { pageId: string; }

export const RevisionHistory: React.FC<RevisionHistoryProps> = ({ pageId }) => {
  const [revisions, setRevisions] = useState<Revision[]>([]);
  const [expanded, setExpanded]   = useState(false);
  const [loading, setLoading]     = useState(false);
  const [error, setError]         = useState('');

  const load = async () => {
    if (loading) return;
    setLoading(true);
    setError('');
    try {
      const data = await apiFetch(`/pages/${pageId}/revisions`);
      setRevisions(data || []);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleToggle = () => {
    const next = !expanded;
    setExpanded(next);
    if (next && revisions.length === 0) load();
  };

  return (
    <div>
      <button
        onClick={handleToggle}
        style={{
          display: 'inline-flex', alignItems: 'center', gap: 6,
          background: 'none', border: 'none',
          color: 'var(--gray-500)', cursor: 'pointer',
          fontSize: 12, fontWeight: 600, padding: '4px 0',
          letterSpacing: '.02em',
          transition: 'color 150ms',
        }}
        onMouseEnter={e => (e.currentTarget.style.color = 'var(--primary)')}
        onMouseLeave={e => (e.currentTarget.style.color = 'var(--gray-500)')}
      >
        <span>{expanded ? '▲' : '▼'}</span>
        REVISION HISTORY
      </button>

      {expanded && (
        <div style={{ marginTop: 10 }}>
          {loading && (
            <p style={{ color: 'var(--gray-400)', fontSize: 12 }}>Loading…</p>
          )}
          {error && (
            <p style={{ color: 'var(--danger)', fontSize: 12 }}>⚠ {error}</p>
          )}
          {!loading && revisions.length === 0 && !error && (
            <p style={{ color: 'var(--gray-400)', fontSize: 12 }}>No revisions found.</p>
          )}
          {revisions.length > 0 && (
            <table style={{
              width: '100%', fontSize: 12,
              borderCollapse: 'collapse',
            }}>
              <thead>
                <tr style={{ borderBottom: '1px solid var(--gray-200)' }}>
                  {['Version', 'Saved by', 'When'].map(h => (
                    <th key={h} style={{
                      padding: '6px 10px', textAlign: 'left',
                      color: 'var(--gray-400)', fontWeight: 600, letterSpacing: '.04em',
                      fontSize: 11, textTransform: 'uppercase',
                    }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {revisions.map((rev, i) => (
                  <tr
                    key={rev.id}
                    style={{
                      borderBottom: '1px solid var(--gray-100)',
                      background: i === 0 ? 'var(--primary-bg)' : 'transparent',
                    }}
                  >
                    <td style={{ padding: '7px 10px' }}>
                      <span className="badge badge-blue">v{rev.version}</span>
                    </td>
                    <td style={{ padding: '7px 10px', fontFamily: 'var(--font-mono)', color: 'var(--gray-600)', fontSize: 11 }}>
                      {rev.created_by.substring(0, 8)}…
                    </td>
                    <td style={{ padding: '7px 10px', color: 'var(--gray-500)' }}>
                      {new Date(rev.created_at).toLocaleString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  );
};
