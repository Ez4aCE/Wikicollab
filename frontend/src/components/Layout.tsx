import React from 'react';
import { Outlet, Link, useNavigate, useLocation } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

export const Layout: React.FC = () => {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();

  const handleLogout = async () => {
    try { await logout(); navigate('/login'); }
    catch (err) { console.error('Logout failed', err); }
  };

  const isPageEditor = location.pathname.includes('/page/');

  return (
    <div style={{ display: 'flex', flexDirection: 'column', minHeight: '100vh' }}>
      {/* ── Navbar ── */}
      <header style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '0 24px', height: 56,
        background: 'linear-gradient(135deg, #16a34a 0%, #059669 100%)',
        boxShadow: '0 2px 8px rgba(22,163,74,.35)',
        position: 'sticky', top: 0, zIndex: 100,
        flexShrink: 0,
      }}>
        {/* Brand */}
        <Link to="/dashboard" style={{
          display: 'flex', alignItems: 'center', gap: 10,
          color: '#fff', textDecoration: 'none',
        }}>
          <span style={{
            width: 30, height: 30, borderRadius: 8,
            background: 'rgba(255,255,255,.2)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: 16, fontWeight: 800,
          }}>W</span>
          <span style={{ fontWeight: 700, fontSize: 16, letterSpacing: '-0.02em' }}>
            WikiCollab
          </span>
        </Link>

        {/* Right side */}
        {user && (
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            {/* Avatar initials */}
            <div style={{
              width: 32, height: 32, borderRadius: '50%',
              background: 'rgba(255,255,255,.25)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontWeight: 700, fontSize: 13, color: '#fff',
              border: '2px solid rgba(255,255,255,.4)',
            }}>
              {user.username?.[0]?.toUpperCase() || 'U'}
            </div>

            {/* Username */}
            <span style={{ color: 'rgba(255,255,255,.85)', fontSize: 13, fontWeight: 500,
              display: 'none',
            }} className="nav-username">
              {user.username}
            </span>

            {/* Logout */}
            <button onClick={handleLogout} style={{
              padding: '5px 14px', background: 'rgba(255,255,255,.15)',
              border: '1.5px solid rgba(255,255,255,.35)',
              color: '#fff', borderRadius: 6, cursor: 'pointer',
              fontSize: 13, fontWeight: 600,
              transition: 'background 150ms',
            }}
              onMouseEnter={e => (e.currentTarget.style.background = 'rgba(255,255,255,.25)')}
              onMouseLeave={e => (e.currentTarget.style.background = 'rgba(255,255,255,.15)')}
            >
              Sign out
            </button>
          </div>
        )}
      </header>

      {/* ── Main content ── */}
      <main style={{
        flex: 1,
        background: isPageEditor ? '#d1d5db' : 'var(--gray-50)',
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',     // editor canvas handles its own scroll
        minHeight: 0,           // critical: allows flex children to shrink below content size
      }}>
        <Outlet />
      </main>
    </div>
  );
};
