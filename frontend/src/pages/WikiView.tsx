import React, { useEffect, useState } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { apiFetch } from '../api/client';

interface PageItem { id: string; title: string; wiki_id: string; }

export const WikiView: React.FC = () => {
  const { wikiID } = useParams<{ wikiID: string }>();
  const [pages, setPages]           = useState<PageItem[]>([]);
  const [wikiName, setWikiName]     = useState('');
  const [newPageTitle, setNewPageTitle] = useState('');
  const [error, setError]           = useState('');
  const [creating, setCreating]     = useState(false);
  const navigate = useNavigate();

  // Share
  const [shareCode, setShareCode]   = useState<string | null>(null);
  const [shareOpen, setShareOpen]   = useState(false);
  const [copyLabel, setCopyLabel]   = useState('Copy');
  const [shareLoading, setShareLoading] = useState(false);

  const loadData = async () => {
    try {
      const wiki = await apiFetch(`/wikis/${wikiID}`);
      setWikiName(wiki.name);
      const pagesData = await apiFetch(`/wikis/${wikiID}/pages`);
      setPages(pagesData || []);
    } catch (err: any) {
      setError(err.message);
    }
  };

  useEffect(() => { loadData(); }, [wikiID]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newPageTitle.trim()) return;
    setCreating(true);
    try {
      const newPage = await apiFetch(`/wikis/${wikiID}/pages`, {
        method: 'POST',
        body: JSON.stringify({ title: newPageTitle, content: `# ${newPageTitle}\n\nStart writing here...` }),
      });
      navigate(`/wiki/${wikiID}/page/${newPage.id}`);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const handleShare = async () => {
    if (shareOpen) { setShareOpen(false); return; }
    setShareLoading(true);
    try {
      const data = await apiFetch(`/wikis/${wikiID}/share`, { method: 'POST' });
      setShareCode(data.code);
      setShareOpen(true);
      setCopyLabel('Copy');
    } catch (err: any) {
      setError(err.message || 'Failed to generate share code.');
    } finally {
      setShareLoading(false);
    }
  };

  const handleCopy = () => {
    if (!shareCode) return;
    navigator.clipboard.writeText(shareCode).then(() => {
      setCopyLabel('Copied ✓');
      setTimeout(() => setCopyLabel('Copy'), 2000);
    });
  };

  return (
    <div className="page-wrapper">
      {/* ── Back link ── */}
      <Link to="/dashboard" style={{
        display: 'inline-flex', alignItems: 'center', gap: 6,
        fontSize: 13, fontWeight: 500, color: 'var(--gray-500)',
        marginBottom: 20, textDecoration: 'none',
      }}
        onMouseEnter={e => (e.currentTarget.style.color = 'var(--primary)')}
        onMouseLeave={e => (e.currentTarget.style.color = 'var(--gray-500)')}
      >
        ← Dashboard
      </Link>

      {/* ── Wiki title + Share ── */}
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12, marginBottom: shareOpen ? 12 : 24, flexWrap: 'wrap' }}>
        <div style={{
          width: 48, height: 48, borderRadius: 12, flexShrink: 0,
          background: `linear-gradient(135deg, hsl(${(wikiName.charCodeAt(0) * 37) % 360}deg 65% 55%), hsl(${(wikiName.charCodeAt(0) * 37 + 40) % 360}deg 65% 45%))`,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontSize: 22, fontWeight: 800, color: '#fff',
          boxShadow: 'var(--shadow-sm)',
        }}>
          {wikiName[0]?.toUpperCase() || '?'}
        </div>
        <div style={{ flex: 1 }}>
          <h1 style={{ fontSize: 24, fontWeight: 700, color: 'var(--gray-900)', marginBottom: 2 }}>
            {wikiName || '…'}
          </h1>
          <p style={{ fontSize: 13, color: 'var(--gray-400)' }}>
            {pages.length} page{pages.length !== 1 ? 's' : ''}
          </p>
        </div>
        <button
          onClick={handleShare}
          className="btn btn-outline btn-sm"
          disabled={shareLoading}
          style={{ alignSelf: 'flex-start', marginTop: 4 }}
        >
          {shareLoading ? '…' : shareOpen ? '✕ Hide code' : '🔗 Share'}
        </button>
      </div>

      {/* ── Share code panel ── */}
      {shareOpen && shareCode && (
        <div style={{
          display: 'flex', alignItems: 'center',
          background: 'linear-gradient(135deg, #eef2ff, #f5f3ff)',
          border: '1.5px solid #c7d2fe',
          borderRadius: 10, padding: '14px 18px',
          marginBottom: 20, flexWrap: 'wrap', gap: 12,
        }}>
          <div style={{ fontSize: 13, color: 'var(--gray-700)' }}>
            Share this code with your collaborators:
          </div>
          <div style={{
            fontFamily: 'var(--font-mono)', fontSize: 22, fontWeight: 800,
            letterSpacing: '0.2em', color: 'var(--primary)',
            background: '#fff', border: '2px solid #c7d2fe',
            padding: '6px 16px', borderRadius: 8,
          }}>
            {shareCode}
          </div>
          <button
            onClick={handleCopy}
            className="btn btn-primary btn-sm"
          >
            {copyLabel}
          </button>
        </div>
      )}

      {/* ── Error ── */}
      {error && (
        <div className="alert alert-error" style={{ marginBottom: 20 }}>
          <span>⚠</span> {error}
          <button onClick={() => setError('')} style={{ marginLeft: 'auto', background: 'none', border: 'none', cursor: 'pointer', color: 'var(--danger)', fontSize: 16 }}>✕</button>
        </div>
      )}

      {/* ── Create page card ── */}
      <div className="card" style={{ marginBottom: 20 }}>
        <div style={{ fontWeight: 700, fontSize: 15, color: 'var(--gray-900)', marginBottom: 12 }}>
          ✦ New Page
        </div>
        <form onSubmit={handleCreate} style={{ display: 'flex', gap: 10 }}>
          <input
            className="form-input"
            type="text"
            placeholder="Page title…"
            value={newPageTitle}
            onChange={e => setNewPageTitle(e.target.value)}
            style={{ flex: 1 }}
          />
          <button
            type="submit"
            className="btn btn-success btn-sm"
            disabled={creating}
            style={{ flexShrink: 0 }}
          >
            {creating ? 'Creating…' : 'Create Page'}
          </button>
        </form>
      </div>

      {/* ── Pages list ── */}
      <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
        <div style={{ padding: '14px 20px', borderBottom: '1px solid var(--gray-200)' }}>
          <h2 style={{ fontWeight: 700, fontSize: 15, color: 'var(--gray-900)' }}>
            Pages
            {pages.length > 0 && (
              <span className="badge badge-blue" style={{ marginLeft: 10 }}>{pages.length}</span>
            )}
          </h2>
        </div>

        {pages.length === 0 ? (
          <div style={{ padding: '40px 20px', textAlign: 'center' }}>
            <div style={{ fontSize: 36, marginBottom: 10 }}>📄</div>
            <p style={{ color: 'var(--gray-400)', fontSize: 14 }}>
              No pages yet. Create the first one above.
            </p>
          </div>
        ) : (
          <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
            {pages.map((page, i) => (
              <li key={page.id} style={{
                borderBottom: i < pages.length - 1 ? '1px solid var(--gray-100)' : 'none',
              }}>
                <Link
                  to={`/wiki/${wikiID}/page/${page.id}`}
                  style={{
                    display: 'flex', alignItems: 'center', gap: 14,
                    padding: '13px 20px', textDecoration: 'none',
                    transition: 'background 150ms',
                  }}
                  onMouseEnter={e => (e.currentTarget.style.background = 'var(--gray-50)')}
                  onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                >
                  <span style={{
                    width: 30, height: 30, borderRadius: 8, flexShrink: 0,
                    background: 'var(--primary-bg)',
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    fontSize: 14,
                  }}>📝</span>
                  <span style={{
                    flex: 1, fontWeight: 600, fontSize: 14, color: 'var(--gray-900)',
                    whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                  }}>
                    {page.title}
                  </span>
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
