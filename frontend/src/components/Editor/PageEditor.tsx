import React, { useEffect, useRef, useState, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { useCollaboration } from '../../hooks/useCollaboration';
import type { Block } from '../../hooks/useCollaboration';
import { BlockEditor } from './BlockEditor';
import { RevisionHistory } from './RevisionHistory';
import { apiFetch } from '../../api/client';

const ROWS_PER_PAGE = 15;

interface PageEditorProps {
  pageId: string;
  wikiId: string;
  currentUserId: string;
  initialTitle: string;
  initialMarkdown: string;
  initialVersion: number;
}

type SaveStatus = 'saved' | 'saving' | 'unsaved';

export const PageEditor: React.FC<PageEditorProps> = ({
  pageId, wikiId, currentUserId,
  initialTitle, initialMarkdown, initialVersion,
}) => {
  const [title, setTitle]               = useState(initialTitle);
  const [saveStatus, setSaveStatus]     = useState<SaveStatus>('saved');
  const [showRevisions, setShowRevisions] = useState(false);

  const [shareCode, setShareCode]       = useState<string | null>(null);
  const [shareOpen, setShareOpen]       = useState(false);
  const [shareError, setShareError]     = useState<string | null>(null);
  const [copyLabel, setCopyLabel]       = useState('Copy');
  const [shareLoading, setShareLoading] = useState(false);

  const {
    blocks, locks, version, error,
    initializePage, acquireLock, releaseLock,
    addPage, updateBlockContent, doSave, setTitleRef, dismissError,
  } = useCollaboration(pageId, currentUserId);

  // ── Initialize ────────────────────────────────────────────────────────────
  useEffect(() => {
    initializePage(initialMarkdown, initialVersion);
  }, [initialMarkdown, initialVersion, initializePage]);

  // Keep hook's title ref in sync whenever title changes
  useEffect(() => { setTitleRef(title); }, [title, setTitleRef]);

  // ── Auto-save — triggered ONLY by user actions, never by remote updates ───
  // This prevents the infinite loop: remote update → setBlocks → effect → save
  // → page_updated broadcast → remote update → ...
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const scheduleAutoSave = useCallback(() => {
    setSaveStatus('unsaved');
    if (saveTimer.current) clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(() => {
      setSaveStatus('saving');
      doSave();
    }, 2000);
  }, [doSave]);

  // Mark as saved when server confirms with a new version
  const prevVersion = useRef(version);
  useEffect(() => {
    if (version !== prevVersion.current) {
      prevVersion.current = version;
      setSaveStatus('saved');
    }
  }, [version]);

  // ── Handlers ──────────────────────────────────────────────────────────────
  // Wrap updateBlockContent so typing → scheduleAutoSave
  const handleContentChange = useCallback((id: string, content: string) => {
    updateBlockContent(id, content);
    scheduleAutoSave();
  }, [updateBlockContent, scheduleAutoSave]);

  const handleTitleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setTitle(e.target.value);
    scheduleAutoSave();
  };

  // ── Share ─────────────────────────────────────────────────────────────────
  const handleShare = async () => {
    if (shareOpen) { setShareOpen(false); setShareError(null); return; }
    setShareLoading(true);
    setShareError(null);
    try {
      const data = await apiFetch(`/wikis/${wikiId}/share`, { method: 'POST' });
      setShareCode(data.code);
      setShareOpen(true);
      setCopyLabel('Copy');
    } catch (err: any) {
      setShareError(err.message?.toLowerCase().includes('forbidden')
        ? 'Only the wiki owner can generate a share code.'
        : (err.message || 'Could not generate share code.'));
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

  // ── Add page ──────────────────────────────────────────────────────────────
  const canvasRef = useRef<HTMLDivElement>(null);
  const handleAddPage = () => {
    addPage(ROWS_PER_PAGE);
    scheduleAutoSave();
    setTimeout(() => {
      canvasRef.current?.scrollTo({ top: canvasRef.current.scrollHeight, behavior: 'smooth' });
    }, 80);
  };

  // ── Page grouping ─────────────────────────────────────────────────────────
  const pages: Block[][] = [];
  for (let i = 0; i < Math.max(blocks.length, ROWS_PER_PAGE); i += ROWS_PER_PAGE) {
    pages.push(blocks.slice(i, i + ROWS_PER_PAGE));
  }

  const statusText: Record<SaveStatus, string> = {
    saved: '✓ Saved', saving: 'Saving…', unsaved: '● Unsaved',
  };

  // ── Render ────────────────────────────────────────────────────────────────
  return (
    <div style={{
      display: 'flex', flexDirection: 'column',
      flex: 1, minHeight: 0, overflow: 'hidden',
      background: '#fff',
    }}>

      {/* ── Toolbar ── */}
      <div style={{
        display: 'flex', alignItems: 'center', gap: 10,
        padding: '0 14px', height: 44, flexShrink: 0,
        background: 'linear-gradient(135deg, #16a34a 0%, #059669 100%)',
        boxShadow: '0 2px 6px rgba(22,163,74,.3)',
      }}>
        <Link to={`/wiki/${wikiId}`} style={{
          color: 'rgba(255,255,255,.9)', fontSize: 13, fontWeight: 600,
          textDecoration: 'none', padding: '3px 9px', borderRadius: 5,
          border: '1.5px solid rgba(255,255,255,.28)',
          background: 'rgba(255,255,255,.12)',
        }}>← Wiki</Link>

        <span style={{ color: 'rgba(255,255,255,.2)' }}>|</span>

        <span style={{
          color: '#fff', fontWeight: 600, fontSize: 12,
          background: 'rgba(255,255,255,.18)', borderRadius: 99, padding: '2px 10px',
        }}>
          {statusText[saveStatus]}
        </span>
        <span style={{ color: 'rgba(255,255,255,.45)', fontSize: 11 }}>v{version}</span>

        <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ color: 'rgba(255,255,255,.35)', fontSize: 11 }}>
            {pages.length}p · {ROWS_PER_PAGE}rows
          </span>
          <span style={{ color: 'rgba(255,255,255,.2)' }}>|</span>
          <button onClick={handleShare} disabled={shareLoading} style={{
            fontSize: 12, padding: '3px 10px',
            border: '1.5px solid rgba(255,255,255,.35)', borderRadius: 5,
            cursor: 'pointer',
            background: shareOpen ? 'rgba(255,255,255,.3)' : 'rgba(255,255,255,.14)',
            color: '#fff', fontWeight: 600,
          }}>
            {shareLoading ? '…' : '🔗 Share'}
          </button>
          <button onClick={() => setShowRevisions(v => !v)} style={{
            fontSize: 12, padding: '3px 10px',
            border: '1.5px solid rgba(255,255,255,.35)', borderRadius: 5,
            cursor: 'pointer',
            background: showRevisions ? 'rgba(255,255,255,.3)' : 'rgba(255,255,255,.14)',
            color: '#fff', fontWeight: 600,
          }}>
            📋 History
          </button>
        </div>
      </div>

      {/* ── Active editors banner ── */}
      {Object.keys(locks).length > 0 && (
        <div style={{
          display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap',
          padding: '5px 14px', flexShrink: 0,
          background: '#fefce8', borderBottom: '1px solid #fde68a', fontSize: 12,
        }}>
          <span style={{ color: '#92400e', fontWeight: 700 }}>✏️ Active editors:</span>
          {Object.entries(locks).map(([bid, info]) => (
            <span key={bid} style={{
              display: 'inline-flex', alignItems: 'center', gap: 4,
              background: info.isMe ? '#dcfce7' : '#fde68a',
              border: `1px solid ${info.isMe ? '#86efac' : '#fbbf24'}`,
              borderRadius: 99, padding: '2px 8px',
              color: info.isMe ? '#166534' : '#92400e', fontWeight: 600,
            }}>
              <span style={{
                width: 16, height: 16, borderRadius: '50%',
                background: info.isMe ? '#16a34a' : '#f59e0b',
                color: '#fff', fontSize: 9, fontWeight: 800,
                display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
              }}>
                {info.username.charAt(0).toUpperCase()}
              </span>
              {info.isMe ? 'You' : info.username} — row {parseInt(bid.replace('block-', '')) + 1}
            </span>
          ))}
        </div>
      )}

      {/* ── Share panel ── */}
      {shareOpen && shareCode && (
        <div style={{
          display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 12,
          padding: '10px 18px', flexShrink: 0,
          background: 'linear-gradient(135deg, #f0fdf4, #ecfdf5)',
          borderBottom: '1.5px solid #bbf7d0',
        }}>
          <span style={{ fontSize: 13, color: '#166534', fontWeight: 500 }}>
            🔗 Share this invite code:
          </span>
          <span style={{
            fontFamily: 'monospace', fontSize: 22, fontWeight: 800,
            letterSpacing: '0.22em', color: '#15803d',
            background: '#fff', border: '2px solid #86efac',
            padding: '4px 16px', borderRadius: 8,
          }}>
            {shareCode}
          </span>
          <button onClick={handleCopy} className="btn btn-primary btn-sm">{copyLabel}</button>
          <button onClick={() => setShareOpen(false)} style={{
            marginLeft: 'auto', background: 'none', border: 'none',
            cursor: 'pointer', color: '#16a34a', fontSize: 20,
          }}>✕</button>
        </div>
      )}

      {/* ── Share error ── */}
      {shareError && (
        <div style={{
          background: '#fef2f2', borderBottom: '1px solid #fecaca',
          padding: '7px 16px', fontSize: 13, flexShrink: 0,
          display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        }}>
          <span style={{ color: '#dc2626' }}>🔒 {shareError}</span>
          <button onClick={() => setShareError(null)} style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#dc2626', fontSize: 18 }}>✕</button>
        </div>
      )}

      {/* ── WebSocket error ── */}
      {error && (
        <div style={{
          background: '#fef2f2', borderBottom: '1px solid #fecaca',
          padding: '7px 16px', fontSize: 13, flexShrink: 0,
          display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        }}>
          <span style={{ color: '#dc2626' }}>⚠ {error}</span>
          <button onClick={dismissError} style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#dc2626', fontSize: 18 }}>✕</button>
        </div>
      )}

      {/* ── Document canvas ── */}
      <div ref={canvasRef} style={{
        flex: 1, minHeight: 0,
        overflowY: 'auto', overflowX: 'hidden',
        background: '#d1d5db',
        padding: '32px 20px 48px',
      }}>
        {pages.map((pageBlocks, pageIndex) => (
          <React.Fragment key={pageIndex}>

            {/* Page card */}
            <div style={{
              background: '#ffffff', maxWidth: 700,
              margin: '0 auto 8px', borderRadius: 3,
              boxShadow: '0 2px 10px rgba(0,0,0,.18), 0 0 0 1px rgba(0,0,0,.06)',
              // overflow: hidden removed — would clip row scrollbars for readers
            }}>
              {/* Title — page 1 only */}
              {pageIndex === 0 && (
                <div style={{
                  borderBottom: '2px solid #d1fae5', background: '#f0fdf4',
                  padding: '14px 20px 14px 58px',
                }}>
                  <input
                    type="text"
                    value={title}
                    onChange={handleTitleChange}
                    placeholder="Untitled document"
                    style={{
                      width: '100%', border: 'none', outline: 'none',
                      fontSize: 20, fontWeight: 700, color: '#166534',
                      background: 'transparent', fontFamily: 'inherit',
                    }}
                  />
                </div>
              )}

              {/* Page header */}
              <div style={{ display: 'flex', height: 24, background: '#f0fdf4', borderBottom: '1px solid #d1fae5' }}>
                <div style={{
                  width: 48, minWidth: 48, borderRight: '1px solid #d1fae5',
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  fontSize: 10, color: '#a7f3d0', fontWeight: 700,
                }}>#</div>
                <div style={{
                  paddingLeft: 12, display: 'flex', alignItems: 'center',
                  fontSize: 10, color: '#16a34a', fontWeight: 600,
                  letterSpacing: 2, textTransform: 'uppercase',
                }}>
                  Page {pageIndex + 1} of {pages.length}
                </div>
              </div>

              {/* Rows */}
              {pageBlocks.map((block, localIdx) => {
                const absRow = pageIndex * ROWS_PER_PAGE + localIdx;
                return (
                  <BlockEditor
                    key={block.id}
                    block={block}
                    rowIndex={absRow + 1}
                    lockedBy={locks[block.id]?.username || null}
                    isOwnedByMe={locks[block.id]?.isMe || false}
                    onAcquireLock={acquireLock}
                    onReleaseLock={releaseLock}
                    onUpdateContent={handleContentChange}
                  />
                );
              })}

              {/* Page footer */}
              <div style={{
                height: 20, background: '#f9fffe', borderTop: '1px solid #d1fae5',
                display: 'flex', alignItems: 'center', justifyContent: 'flex-end',
                paddingRight: 16, fontSize: 10, color: '#a7f3d0',
              }}>
                {pageIndex + 1} / {pages.length}
              </div>
            </div>

            {/* Gap between pages */}
            {pageIndex < pages.length - 1 && (
              <div style={{
                maxWidth: 700, margin: '0 auto 8px', height: 22,
                display: 'flex', alignItems: 'center', gap: 10,
              }}>
                <div style={{ flex: 1, height: 1, background: 'rgba(0,0,0,.12)' }} />
                <span style={{ fontSize: 10, color: 'rgba(255,255,255,.45)', fontWeight: 500 }}>
                  Page {pageIndex + 2}
                </span>
                <div style={{ flex: 1, height: 1, background: 'rgba(0,0,0,.12)' }} />
              </div>
            )}
          </React.Fragment>
        ))}

        {/* Add Page button */}
        <div style={{ textAlign: 'center', padding: '20px 0 8px', maxWidth: 700, margin: '0 auto' }}>
          <button onClick={handleAddPage} style={{
            display: 'inline-flex', alignItems: 'center', gap: 8,
            padding: '9px 22px',
            background: 'linear-gradient(135deg, #16a34a, #059669)',
            color: '#fff', border: 'none', borderRadius: 8,
            fontSize: 14, fontWeight: 600, cursor: 'pointer',
            boxShadow: '0 2px 8px rgba(22,163,74,.35)',
          }}>
            <span style={{ fontSize: 18 }}>+</span> Add Page
          </button>
        </div>
      </div>

      {/* ── History drawer ── */}
      {showRevisions && (
        <div style={{
          borderTop: '2px solid #d1fae5', background: '#f0fdf4',
          maxHeight: 220, overflowY: 'auto',
          padding: '10px 16px', flexShrink: 0,
        }}>
          <RevisionHistory pageId={pageId} />
        </div>
      )}
    </div>
  );
};
