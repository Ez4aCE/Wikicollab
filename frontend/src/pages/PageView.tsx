import React, { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { apiFetch } from '../api/client';
import { PageEditor } from '../components/Editor/PageEditor';
import { useAuth } from '../context/AuthContext';

interface PageData {
  id: string;
  wiki_id: string;
  title: string;
  content: string;
  version: number;
}

export const PageView: React.FC = () => {
  const { wikiID, pageID } = useParams<{ wikiID: string; pageID: string }>();
  const [page, setPage]   = useState<PageData | null>(null);
  const [error, setError] = useState('');
  const { user } = useAuth();

  useEffect(() => {
    const fetchPage = async () => {
      try {
        const data = await apiFetch(`/pages/${pageID}`);
        setPage(data);
      } catch (err: any) {
        setError(err.message);
      }
    };
    fetchPage();
  }, [pageID]);

  if (error) return (
    <div style={{ maxWidth: 800, margin: '40px auto', padding: 20 }}>
      <div className="alert alert-error" style={{ marginBottom: 16 }}>⚠ {error}</div>
      <Link to={`/wiki/${wikiID}`} className="btn btn-outline btn-sm">← Back to Wiki</Link>
    </div>
  );

  if (!page || !user) return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', flex: 1, color: 'var(--gray-400)' }}>
      Loading…
    </div>
  );

  // ── CRITICAL: this div must carry flex:1 so PageEditor's flex layout fills the screen ──
  return (
    <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0, overflow: 'hidden' }}>
      <PageEditor
        pageId={page.id}
        wikiId={wikiID!}
        currentUserId={user.id}
        initialTitle={page.title}
        initialMarkdown={page.content}
        initialVersion={page.version}
      />
    </div>
  );
};
