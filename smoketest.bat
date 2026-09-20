@echo off
setlocal EnableDelayedExpansion

set CURL=C:\Windows\System32\curl.exe
set BASE=http://localhost:8080

echo.
echo ==========================================
echo  WikiCollab Live API Smoke Tests
echo ==========================================

echo.
echo [1] Health check
%CURL% -s %BASE%/health
echo.

echo [2] Register Bob
%CURL% -s -X POST %BASE%/api/register -H "Content-Type: application/json" -d "{\"username\":\"Bob\",\"email\":\"bob@example.com\",\"password\":\"TestPassword123!\"}"
echo.

echo [3] Duplicate register Bob (expect 409)
%CURL% -s -o NUL -w "HTTP Status: %%{http_code}" -X POST %BASE%/api/register -H "Content-Type: application/json" -d "{\"username\":\"Bob\",\"email\":\"bob@example.com\",\"password\":\"TestPassword123!\"}"
echo.

echo [4] Login Alice (already registered in prev run)
%CURL% -s -X POST %BASE%/api/login -H "Content-Type: application/json" -d "{\"email\":\"alice@example.com\",\"password\":\"TestPassword123!\"}" -c cookies_alice.txt
echo.

echo [5] /api/me as Alice
%CURL% -s %BASE%/api/me -b cookies_alice.txt
echo.

echo [6] Invalid credentials (expect 401)
%CURL% -s -o NUL -w "HTTP Status: %%{http_code}" -X POST %BASE%/api/login -H "Content-Type: application/json" -d "{\"email\":\"alice@example.com\",\"password\":\"wrongpassword\"}"
echo.

echo [7] Create wiki as Alice
%CURL% -s -X POST %BASE%/api/wikis -H "Content-Type: application/json" -d "{\"name\":\"Computer Science Wiki\"}" -b cookies_alice.txt -o wiki.json
type wiki.json
echo.

echo [8] List wikis
%CURL% -s %BASE%/api/wikis -b cookies_alice.txt
echo.

echo [9] Get wiki ID then create page
for /f "tokens=2 delims=:," %%i in ('findstr /r "\"id\":\"[^\"]*\"" wiki.json') do (
  set WIKIID=%%~i
  set WIKIID=!WIKIID:"=!
  goto :gotid
)
:gotid
echo Wiki ID: !WIKIID!

%CURL% -s -X POST %BASE%/api/wikis/!WIKIID!/pages -H "Content-Type: application/json" -d "{\"title\":\"Introduction to Algorithms\",\"content\":\"# Introduction\n\nThis is a paragraph.\n\n## Algorithms\n\n- Sorting\n- Searching\"}" -b cookies_alice.txt -o page.json
type page.json
echo.

echo [10] Update page (expected_version=1)
for /f "tokens=2 delims=:," %%i in ('findstr /r "\"id\":\"[^\"]*\"" page.json') do (
  set PAGEID=%%~i
  set PAGEID=!PAGEID:"=!
  goto :gotpageid
)
:gotpageid
echo Page ID: !PAGEID!

%CURL% -s -X PUT %BASE%/api/pages/!PAGEID! -H "Content-Type: application/json" -d "{\"title\":\"Introduction to Algorithms\",\"content\":\"# Introduction\n\nUpdated paragraph.\",\"expected_version\":1}" -b cookies_alice.txt
echo.

echo [11] Stale version conflict (expected_version=1 again, expect 409)
%CURL% -s -o NUL -w "HTTP Status: %%{http_code}" -X PUT %BASE%/api/pages/!PAGEID! -H "Content-Type: application/json" -d "{\"title\":\"Intro\",\"content\":\"stale\",\"expected_version\":1}" -b cookies_alice.txt
echo.

echo [12] Get revisions
%CURL% -s %BASE%/api/pages/!PAGEID!/revisions -b cookies_alice.txt
echo.

echo [13] Login Bob
%CURL% -s -X POST %BASE%/api/login -H "Content-Type: application/json" -d "{\"email\":\"bob@example.com\",\"password\":\"TestPassword123!\"}" -c cookies_bob.txt
echo.

echo [14] Bob accessing Alice wiki (expect 403)
%CURL% -s -o NUL -w "HTTP Status: %%{http_code}" %BASE%/api/wikis/!WIKIID! -b cookies_bob.txt
echo.

echo [15] Unauthenticated /api/me (expect 401)
%CURL% -s -o NUL -w "HTTP Status: %%{http_code}" %BASE%/api/me
echo.

echo [16] Logout Alice
%CURL% -s -X POST %BASE%/api/logout -b cookies_alice.txt -c cookies_alice.txt
echo.

echo [17] /api/me after logout (expect 401)
%CURL% -s -o NUL -w "HTTP Status: %%{http_code}" %BASE%/api/me -b cookies_alice.txt
echo.

echo.
echo ==========================================
echo  ALL SMOKE TESTS COMPLETE
echo ==========================================
