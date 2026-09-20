import { useState, useEffect, useRef, useCallback } from 'react';
import type { InboundMessage, OutboundMessage } from '../types/collaboration';
import { WS_BASE } from '../api/client';

// ── Block separator ────────────────────────────────────────────────────────
// We use ASCII SOH (\x01) instead of \n so that users can freely press Enter
// inside a cell. \x01 is a non-printable control character that can never be
// typed in a textarea or appear in normal pasted text.
const BLOCK_SEP = '\x01';

const blockId = (index: number) => `block-${index}`;
const ROWS_PER_PAGE = 15;

function padToFullPages(lines: string[]): string[] {
  const targetLen = Math.max(
    ROWS_PER_PAGE,
    Math.ceil(lines.length / ROWS_PER_PAGE) * ROWS_PER_PAGE,
  );
  const result = [...lines];
  while (result.length < targetLen) result.push('');
  return result;
}

export interface Block   { id: string; content: string; }
export interface LockInfo { userId: string; username: string; isMe: boolean; }
export interface LockState { [blockId: string]: LockInfo; }

export function useCollaboration(pageId: string, currentUserId: string) {
  const [blocks,  setBlocks]  = useState<Block[]>([]);
  const [locks,   setLocks]   = useState<LockState>({});
  const [version, setVersion] = useState<number>(1);
  const [error,   setError]   = useState<string | null>(null);

  // Refs — always-current copies so callbacks never go stale
  const wsRef          = useRef<WebSocket | null>(null);
  const locksRef       = useRef<LockState>({});
  const blocksRef      = useRef<Block[]>([]);
  const versionRef     = useRef<number>(1);
  const titleRef       = useRef<string>('');
  const renewIntervals = useRef<{ [id: string]: ReturnType<typeof setInterval> }>({});
  const activeBlockRef = useRef<string | null>(null);

  // ── WebSocket — all logic inline, zero stale-closure risk ─────────────────
  useEffect(() => {
    if (!pageId || !currentUserId) return;

    const ws = new WebSocket(`${WS_BASE}/ws/pages/${pageId}`);
    wsRef.current = ws;

    ws.onopen  = () => setError(null);
    ws.onerror = () => setError('WebSocket connection failed. Check backend.');
    ws.onclose = () => console.log('[WS] closed');

    ws.onmessage = (event) => {
      let msg: InboundMessage;
      try { msg = JSON.parse(event.data) as InboundMessage; }
      catch { console.error('[WS] bad JSON', event.data); return; }

      console.log('[WS] received:', msg.type, msg);   // ← debug: visible in browser DevTools

      switch (msg.type) {

        case 'lock_acquired': {
          const { block_id, user_id, username } = msg;
          if (!block_id || !user_id || !username) break;
          const isMe = user_id === currentUserId;
          console.log('[LOCK] acquired block', block_id, 'by', username, 'isMe:', isMe);
          const next = {
            ...locksRef.current,
            [block_id]: { userId: user_id, username, isMe },
          };
          locksRef.current = next;
          setLocks({ ...next });           // spread to force new reference
          // Start renewal if this lock is ours
          if (isMe) {
            if (renewIntervals.current[block_id]) clearInterval(renewIntervals.current[block_id]);
            renewIntervals.current[block_id] = setInterval(() => {
              wsRef.current?.send(JSON.stringify({ type: 'lock_renew', block_id }));
            }, 5000);
          }
          break;
        }

        case 'lock_denied':
          setError('That row is locked by another user. Wait for them to finish.');
          break;

        case 'lock_released': {
          const { block_id } = msg;
          if (!block_id) break;
          const next = { ...locksRef.current };
          delete next[block_id];
          locksRef.current = next;
          setLocks({ ...next });
          if (renewIntervals.current[block_id]) {
            clearInterval(renewIntervals.current[block_id]);
            delete renewIntervals.current[block_id];
          }
          break;
        }

        case 'page_updated': {
          const { content, version: ver } = msg;
          if (content === undefined || ver === undefined) break;
          versionRef.current = ver;
          setVersion(ver);
          // Merge: keep our locked rows, take server content for everything else
          const incoming = padToFullPages(content.split(BLOCK_SEP));
          const currentLocks = locksRef.current;
          setBlocks(currentBlocks => {
            const next = incoming.map((c, i) => {
              const id  = blockId(i);
              const existing = currentBlocks.find(b => b.id === id);
              if (existing && currentLocks[id]?.isMe) return existing;
              return { id, content: c };
            });
            blocksRef.current = next;
            return next;
          });
          // ↑ intentionally does NOT schedule auto-save — only user actions do
          break;
        }

        case 'error':
          setError(msg.message || 'Server error');
          break;
      }
    };

    return () => {
      Object.values(renewIntervals.current).forEach(clearInterval);
      ws.close();
    };
  // We intentionally re-run this effect only if pageId changes.
  // currentUserId is captured in the closure; it never changes for a logged-in user.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pageId]);

  // ── Initialize from REST ──────────────────────────────────────────────────
  const initializePage = useCallback((markdown: string, ver: number) => {
    versionRef.current = ver;
    setVersion(ver);
    const lines = padToFullPages(markdown.split(BLOCK_SEP));
    const blks  = lines.map((content, i) => ({ id: blockId(i), content }));
    blocksRef.current = blks;
    setBlocks(blks);
  }, []);

  // ── Save via WebSocket ────────────────────────────────────────────────────
  const doSave = useCallback(() => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({
      type:             'page_update',
      title:            titleRef.current || 'Untitled',
      content:          blocksRef.current.map(b => b.content).join(BLOCK_SEP) || ' ',
      expected_version: versionRef.current,
    } as OutboundMessage));
  }, []);

  // ── Lock / unlock ─────────────────────────────────────────────────────────
  const acquireLock = useCallback((id: string) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    const prev = activeBlockRef.current;
    if (prev && prev !== id) {
      // User switched rows — immediately release old lock so collaborators know
      if (renewIntervals.current[prev]) {
        clearInterval(renewIntervals.current[prev]);
        delete renewIntervals.current[prev];
      }
      ws.send(JSON.stringify({ type: 'unlock', block_id: prev } as OutboundMessage));
    }
    activeBlockRef.current = id;
    ws.send(JSON.stringify({ type: 'lock', block_id: id } as OutboundMessage));
  }, []);

  const releaseLock = useCallback((id: string) => {
    // Called by blur timer (10s) or inactivity timer (30s)
    const ws = wsRef.current;
    if (renewIntervals.current[id]) {
      clearInterval(renewIntervals.current[id]);
      delete renewIntervals.current[id];
    }
    if (activeBlockRef.current === id) activeBlockRef.current = null;
    ws?.send(JSON.stringify({ type: 'unlock', block_id: id } as OutboundMessage));
  }, []);

  // ── Update content (called on user keypress only) ─────────────────────────
  const updateBlockContent = useCallback((id: string, content: string) => {
    setBlocks(prev => {
      const next = prev.map(b => b.id === id ? { ...b, content } : b);
      blocksRef.current = next;
      return next;
    });
  }, []);

  // ── Add a full new page ───────────────────────────────────────────────────
  const addPage = useCallback((rowsPerPage: number) => {
    setBlocks(prev => {
      const fill    = prev.length % rowsPerPage === 0 ? 0 : rowsPerPage - (prev.length % rowsPerPage);
      const toAdd   = fill + rowsPerPage;
      const newRows = Array.from({ length: toAdd }, (_, j) => ({
        id: blockId(prev.length + j), content: '',
      }));
      const next = [...prev, ...newRows];
      blocksRef.current = next;
      return next;
    });
  }, []);

  // ── Title ref (kept current for doSave) ───────────────────────────────────
  const setTitleRef = useCallback((t: string) => { titleRef.current = t; }, []);

  return {
    blocks, locks, version, error,
    initializePage, acquireLock, releaseLock,
    addPage, updateBlockContent, doSave, setTitleRef,
    dismissError: () => setError(null),
  };
}
