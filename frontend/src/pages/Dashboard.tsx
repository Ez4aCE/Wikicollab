import React, { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { apiFetch } from '../api/client';

interface Wiki {
  id: string;
  name: string;
  owner_id: string;
  created_at?: string;
}

export const Dashboard: React.FC = () => {
  const { user } = useAuth();
  const [wikis, setWikis]           = useState<Wiki[]>([]);
  const [newWikiName, setNewWikiName] = useState('');
  const [error, setError]           = useState('');
  const [creating, setCreating]     = useState(false);

  // Join-by-code
  const [joinCode, setJoinCode]   = useState('');
  const [joinError, setJoinError] = useState('');
  const [joining, setJoining]     = useState(false);
  const navigate = useNavigate();

  const loadWikis = async () => {
    try {
      const data = await apiFetch('/wikis');
      setWikis(data || []);
    } catch (err: any) {
      setError(err.message);
    }
  };

  useEffect(() => { loadWikis(); }, []);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newWikiName.trim()) return;
    setCreating(true);
    setError('');
    try {
      const wiki = await apiFetch('/wikis', {
        method: 'POST',
        body: JSON.stringify({ name: newWikiName }),
      });
      setNewWikiName('');
      navigate(`/wiki/${wiki.id}`);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const handleJoin = async (e: React.FormEvent) => {
    e.preventDefault();
    const code = joinCode.trim().toUpperCase();
    if (!code) return;
    setJoinError('');
    setJoining(true);
    try {
      const wiki = await apiFetch('/join', {
        method: 'POST',
        body: JSON.stringify({ code }),
      });
      navigate(`/wiki/${wiki.id}`);
    } catch (err: any) {
      setJoinError(err.message || 'Invalid code or join failed.');
    } finally {
      setJoining(false);
    }
  };

  const hour = new Date().getHours();
  const greeting = hour < 12 ? 'Good morning' : hour < 17 ? 'Good afternoon' : 'Good evening';

  return (
    <div className="page-wrapper">
      {/* ── Greeting header ── */}
      <div style={{ marginBottom: 28 }}>
        <h1 style={{ fontSize: 26, fontWeight: 700, color: 'var(--gray-900)', marginBottom: 4 }}>
          {greeting}, {user?.username} 👋
        </h1>
        <p style={{ color: 'var(--gray-500)', fontSize: 14 }}>
          Manage your wikis or join a shared workspace.
        </p>
      </div>

      {error && (
        <div className="alert alert-error" style={{ marginBottom: 20 }}>
          <span>⚠</span> {error}
          <button onClick={() => setError('')} style={{ marginLeft: 'auto', background: 'none', border: 'none', cursor: 'pointer', color: 'var(--danger)', fontSize: 16 }}>✕</button>
        </div>
      )}

      {/* ── Two-column action row ── */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))',
        gap: 16,
        marginBottom: 28,
      }}>
        {/* Create wiki card */}
        <div className="card">
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 16 }}>
            <div style={{
              width: 36, height: 36, borderRadius: 10,
              background: 'linear-gradient(135deg,#4f46e5,#7c3aed)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: 18, color: '#fff', flexShrink: 0,
            }}>+</div>
            <div>
              <div style={{ fontWeight: 700, fontSize: 15, color: 'var(--gray-900)' }}>Create a Wiki</div>
              <div style={{ fontSize: 12, color: 'var(--gray-500)' }}>Start a new knowledge base</div>
            </div>
          </div>
          <form onSubmit={handleCreate} style={{ display: 'flex', gap: 8 }}>
            <input
              className="form-input"
              type="text"
              placeholder="Wiki name…"
              value={newWikiName}
              onChange={e => setNewWikiName(e.target.value)}
              style={{ flex: 1 }}
            />
            <button
              type="submit"
              className="btn btn-primary btn-sm"
              disabled={creating}
              style={{ flexShrink: 0 }}
            >
              {creating ? '…' : 'Create'}
            </button>
          </form>
        </div>

        {/* Join by code card */}
        <div className="card">
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 16 }}>
            <div style={{
              width: 36, height: 36, borderRadius: 10,
              background: 'linear-gradient(135deg,#0ea5e9,#6366f1)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: 18, color: '#fff', flexShrink: 0,
            }}>🔗</div>
            <div>
              <div style={{ fontWeight: 700, fontSize: 15, color: 'var(--gray-900)' }}>Join Shared Wiki</div>
              <div style={{ fontSize: 12, color: 'var(--gray-500)' }}>Enter a 6-character invite code</div>
            </div>
          </div>
          <form onSubmit={handleJoin} style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <div style={{ display: 'flex', gap: 8 }}>
              <input
                className="form-input"
                type="text"
                placeholder="ABC123"
                maxLength={6}
                value={joinCode}
                onChange={e => setJoinCode(e.target.value.toUpperCase())}
                style={{
                  flex: 1, textTransform: 'uppercase',
                  letterSpacing: '0.15em', fontWeight: 700, fontSize: 15,
                  textAlign: 'center', fontFamily: 'var(--font-mono)',
                }}
              />
              <button
                type="submit"
                className="btn btn-sm"
                disabled={joining}
                style={{
                  background: 'linear-gradient(135deg,#0ea5e9,#6366f1)',
                  color: '#fff', border: 'none', flexShrink: 0,
                }}
              >
                {joining ? '…' : 'Join'}
              </button>
            </div>
            {joinError && (
              <span style={{ color: 'var(--danger)', fontSize: 13 }}>⚠ {joinError}</span>
            )}
          </form>
        </div>
      </div>

      {/* ── Wiki list ── */}
      <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
        <div style={{
          padding: '16px 20px', borderBottom: '1px solid var(--gray-200)',
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        }}>
          <h2 style={{ fontSize: 16, fontWeight: 700, color: 'var(--gray-900)' }}>
            Your Wikis
            {wikis.length > 0 && (
              <span className="badge badge-blue" style={{ marginLeft: 10, fontWeight: 600 }}>
                {wikis.length}
              </span>
            )}
          </h2>
          <button className="btn btn-ghost btn-sm" onClick={loadWikis} title="Refresh">⟳</button>
        </div>

        {wikis.length === 0 ? (
          <div style={{ padding: '48px 20px', textAlign: 'center' }}>
            <div style={{ fontSize: 40, marginBottom: 12 }}>📚</div>
            <p style={{ color: 'var(--gray-500)', fontSize: 14 }}>
              No wikis yet. Create one or join a shared wiki above.
            </p>
          </div>
        ) : (
          <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
            {wikis.map((wiki, i) => (
              <li key={wiki.id} style={{
                borderBottom: i < wikis.length - 1 ? '1px solid var(--gray-100)' : 'none',
              }}>
                <Link
                  to={`/wiki/${wiki.id}`}
                  style={{
                    display: 'flex', alignItems: 'center', gap: 14,
                    padding: '14px 20px', textDecoration: 'none',
                    transition: 'background 150ms',
                  }}
                  onMouseEnter={e => (e.currentTarget.style.background = 'var(--gray-50)')}
                  onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                >
                  {/* Wiki icon */}
                  <div style={{
                    width: 38, height: 38, borderRadius: 10, flexShrink: 0,
                    background: `hsl(${(wiki.name.charCodeAt(0) * 37) % 360}deg 70% 90%)`,
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    fontSize: 17, fontWeight: 700,
                    color: `hsl(${(wiki.name.charCodeAt(0) * 37) % 360}deg 50% 35%)`,
                  }}>
                    {wiki.name[0]?.toUpperCase()}
                  </div>

                  {/* Text */}
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{
                      fontWeight: 600, color: 'var(--gray-900)', fontSize: 15,
                      whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                    }}>
                      {wiki.name}
                    </div>
                    {wiki.owner_id === user?.id ? (
                      <div style={{ fontSize: 12, color: 'var(--gray-400)', marginTop: 1 }}>Owner</div>
                    ) : (
                      <div style={{ fontSize: 12, color: '#0ea5e9', marginTop: 1 }}>Shared with you</div>
                    )}
                  </div>

                  {/* Arrow */}
                  <span style={{ color: 'var(--gray-300)', fontSize: 18 }}>›</span>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
};
