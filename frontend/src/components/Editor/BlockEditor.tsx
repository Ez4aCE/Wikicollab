import React, { useRef, useEffect, useCallback } from 'react';
import type { Block } from '../../hooks/useCollaboration';

interface BlockEditorProps {
  block: Block;
  rowIndex: number;
  lockedBy: string | null;
  isOwnedByMe: boolean;
  onAcquireLock: (id: string) => void;
  onReleaseLock: (id: string) => void;
  onUpdateContent: (id: string, content: string) => void;
}

export const ROW_HEIGHT     = 36;   // minimum row height px
const LINE_HEIGHT           = 20;   // px — matches CSS lineHeight
const MAX_LINES             = 10;   // maximum visible lines before scrolling
const CELL_PADDING_V        = 16;   // top + bottom padding (8 each)
export const MAX_ROW_HEIGHT = MAX_LINES * LINE_HEIGHT + CELL_PADDING_V; // 216 px

const BLUR_RELEASE_MS       = 10_000;
const INACTIVITY_RELEASE_MS = 30_000;

// Auto-resize textarea to fit content, capped at MAX_ROW_HEIGHT.
function autoResize(el: HTMLTextAreaElement | null) {
  if (!el) return;
  el.style.height = 'auto';
  el.style.height = `${Math.min(MAX_ROW_HEIGHT, Math.max(ROW_HEIGHT, el.scrollHeight))}px`;
}

export const BlockEditor: React.FC<BlockEditorProps> = ({
  block, rowIndex, lockedBy, isOwnedByMe,
  onAcquireLock, onReleaseLock, onUpdateContent,
}) => {
  const textareaRef      = useRef<HTMLTextAreaElement>(null);
  const blurTimer        = useRef<ReturnType<typeof setTimeout> | null>(null);
  const inactivityTimer  = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clearBlurTimer       = () => { if (blurTimer.current)       { clearTimeout(blurTimer.current);       blurTimer.current = null;       } };
  const clearInactivityTimer = () => { if (inactivityTimer.current) { clearTimeout(inactivityTimer.current); inactivityTimer.current = null; } };

  // ── Auto-resize whenever content changes ───────────────────────────────────
  useEffect(() => {
    if (isOwnedByMe) autoResize(textareaRef.current);
  }, [block.content, isOwnedByMe]);

  // Also resize once on mount when textarea first appears
  useEffect(() => {
    if (isOwnedByMe) {
      // Small delay so the DOM is fully painted
      requestAnimationFrame(() => autoResize(textareaRef.current));
    }
  }, [isOwnedByMe]);

  // ── Inactivity timer (30 s) starts when lock is acquired ───────────────────
  useEffect(() => {
    if (isOwnedByMe) {
      clearInactivityTimer();
      inactivityTimer.current = setTimeout(() => {
        onReleaseLock(block.id);
      }, INACTIVITY_RELEASE_MS);
    } else {
      clearInactivityTimer();
      clearBlurTimer();
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOwnedByMe]);

  // Cleanup on unmount
  useEffect(() => () => { clearBlurTimer(); clearInactivityTimer(); }, []);

  // ── Row styling ───────────────────────────────────────────────────────────
  let bg         = '#ffffff';
  let leftBorder = '3px solid transparent';
  if (isOwnedByMe)  { bg = '#f0fdf4'; leftBorder = '3px solid #16a34a'; }
  else if (lockedBy){ bg = '#fefce8'; leftBorder = '3px solid #f59e0b'; }

  // ── Handlers ──────────────────────────────────────────────────────────────
  const handleClick = () => {
    if (!lockedBy && !isOwnedByMe) onAcquireLock(block.id);
  };

  const handleFocus = () => {
    clearBlurTimer();
    clearInactivityTimer();
    inactivityTimer.current = setTimeout(() => onReleaseLock(block.id), INACTIVITY_RELEASE_MS);
  };

  const handleBlur = () => {
    clearBlurTimer();
    blurTimer.current = setTimeout(() => onReleaseLock(block.id), BLUR_RELEASE_MS);
  };

  const handleChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    onUpdateContent(block.id, e.target.value);
    autoResize(e.target);
    // Reset 30 s inactivity timer on every keystroke
    clearInactivityTimer();
    inactivityTimer.current = setTimeout(() => onReleaseLock(block.id), INACTIVITY_RELEASE_MS);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [block.id, onUpdateContent, onReleaseLock]);

  // ── Gutter icon ───────────────────────────────────────────────────────────
  const renderGutter = () => {
    if (isOwnedByMe) return (
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1 }}>
        <span style={{ fontSize: 13 }}>✏️</span>
      </div>
    );
    if (lockedBy) {
      return (
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1 }}>
          <span style={{ fontSize: 11 }}>🔒</span>
          <div style={{
            width: 16, height: 16, borderRadius: '50%',
            background: '#f59e0b', color: '#fff',
            fontSize: 9, fontWeight: 800,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}>{lockedBy.charAt(0).toUpperCase()}</div>
        </div>
      );
    }
    return <span style={{ fontSize: 10, color: '#a7f3d0' }}>{rowIndex}</span>;
  };

  // ── Lock badge ────────────────────────────────────────────────────────────
  const renderBadge = () => {
    if (!lockedBy || isOwnedByMe) return null;
    return (
      <div style={{
        display: 'flex', alignItems: 'center', gap: 4,
        padding: '2px 8px 2px 4px',
        background: '#fde68a', borderRadius: 99,
        border: '1px solid #fbbf24',
        fontSize: 11, fontWeight: 600, color: '#92400e',
        whiteSpace: 'nowrap', flexShrink: 0, marginLeft: 6, alignSelf: 'flex-start',
        marginTop: 8,
      }}>
        <div style={{
          width: 16, height: 16, borderRadius: '50%',
          background: '#f59e0b', color: '#fff', fontSize: 9, fontWeight: 800,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
          {lockedBy.charAt(0).toUpperCase()}
        </div>
        🔒 {lockedBy}
      </div>
    );
  };

  // ── Render ────────────────────────────────────────────────────────────────
  return (
    <div style={{
      display: 'flex',
      minHeight: ROW_HEIGHT,           // ← minHeight instead of fixed height
      borderBottom: '1px solid #f0fdf4',
      background: bg,
      borderLeft: leftBorder,
      flexShrink: 0,
      alignItems: 'flex-start',        // gutter aligns to top when row expands
    }}>
      {/* Gutter */}
      <div style={{
        width: 48, minWidth: 48,
        minHeight: ROW_HEIGHT,
        borderRight: '1px solid #d1fae5',
        background: isOwnedByMe ? '#dcfce7' : lockedBy ? '#fef3c7' : '#f9fffe',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        userSelect: 'none', flexShrink: 0,
        paddingTop: 8,
      }}>
        {renderGutter()}
      </div>

      {/* Cell */}
      <div style={{
        flex: 1, display: 'flex', alignItems: 'flex-start',
        paddingLeft: 10, paddingRight: 8,
        paddingTop: 0,
        cursor: (!lockedBy && !isOwnedByMe) ? 'text' : 'default',
        minHeight: ROW_HEIGHT,
      }}
        onClick={!isOwnedByMe ? handleClick : undefined}
      >
        {isOwnedByMe ? (
          <textarea
            ref={textareaRef}
            value={block.content}
            onChange={handleChange}
            onFocus={handleFocus}
            onBlur={handleBlur}
            autoFocus
            rows={1}
            spellCheck={false}
            style={{
              flex: 1,
              border: 'none', outline: 'none',
              background: 'transparent',
              resize: 'none',
              overflowY: 'auto',           // scrollbar only when > 10 lines
              minHeight: ROW_HEIGHT,
              maxHeight: MAX_ROW_HEIGHT,   // cap at 10 lines
              padding: '8px 0',
              margin: 0,
              fontFamily: "'Segoe UI', system-ui, sans-serif",
              fontSize: 13.5,
              color: '#166534',
              lineHeight: '20px',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
              boxSizing: 'border-box',
              width: '100%',
            }}
          />
        ) : (
          <div style={{
            flex: 1,
            fontFamily: "'Segoe UI', system-ui, sans-serif",
            fontSize: 13.5,
            color: lockedBy ? '#92400e' : '#1f2328',
            lineHeight: '20px',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
            padding: '8px 0',
            minHeight: ROW_HEIGHT,
            maxHeight: MAX_ROW_HEIGHT,     // same cap on read-only view
            overflowY: 'auto',
          }}>
            {block.content || <span style={{ color: '#d1d5db', fontSize: 12 }}>Empty line</span>}
          </div>
        )}

        {/* Lock badge */}
        {renderBadge()}
      </div>
    </div>
  );
};
