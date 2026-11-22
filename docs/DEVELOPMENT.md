# Developing Googleplex

One desktop app for your Google Tasks, Gmail, Calendar, YouTube, Drive, Docs and Sheets, talking to Google through **your own** Google Cloud project. Go + Wails v2, React (plain JS) with Radix and Tailwind, SQLite. Everything stays on your computer: sign-in secrets in the system keychain, caches in a local SQLite file.

See [WRAPUP.md](WRAPUP.md) for what each section does and its known limits, and the [main README](../README.md) for downloads. Run every command below from the repository root.

## Build an installer

Each script checks its tools first and says what's missing.

| OS | Command | Output in `build/bin/` |
|---|---|---|
| macOS (Apple Silicon) | `./scripts/build-mac.sh` | `Googleplex.app`, `Googleplex-mac-arm64.dmg` |
| Windows 10/11 (64-bit) | `.\scripts\build-windows.ps1` | `googleplex.exe`, `Googleplex-amd64-installer.exe` (with NSIS) |
| Linux (64-bit) | `./scripts/build-linux.sh` | `googleplex`, `googleplex_1.0.0_amd64.deb` (with dpkg-deb) |

You need Go 1.26+ and Node 20+ everywhere. The scripts install the Wails CLI (v2.17.1) if it's missing. Then:

- **macOS:** the Xcode command line tools (`xcode-select --install`). The app is signed ad hoc, so it runs on your own Mac. On another Mac, right-click it and choose **Open** once. Notarizing with an Apple Developer ID is optional for personal use.
- **Windows:** NSIS for the installer (`winget install NSIS.NSIS`, then a new terminal). Without it you get just the exe. The installer fetches Microsoft's WebView2 runtime if the machine doesn't have it; Windows 11 already does. The exe isn't code-signed, so SmartScreen may ask once: **More info → Run anyway**.
- **Linux:** `sudo apt install build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev` (Fedora: `gtk3-devel webkit2gtk4.1-devel`). The build uses WebKitGTK 4.1 (`-tags webkit2_41`).

The Windows exe can also be cross-compiled from a Mac with `wails build -platform windows/amd64`.

## Run it in dev mode

```bash
./scripts/dev-mac.sh        # macOS
./scripts/dev-linux.sh      # Linux
.\scripts\dev-windows.ps1   # Windows
```

## First run

1. **Google Cloud project and sign-in.** The app's setup screen walks you through four steps: create a Cloud project, set up the consent screen (keep it in Testing and add yourself as a test user), create a **Desktop** OAuth client and pick its downloaded JSON, then sign in. The client secret goes to your system keychain.
   Dev builds can skip this: copy `.env.example` to `.env` and fill in the client ID, secret and project ID.
2. **Each feature sets itself up the first time you open it.** If its API is off, the page links to the right Console page and notices by itself once you enable it. Then **Allow access** asks Google for just that feature's permission.

| Section | APIs to enable | Permission asked |
|---|---|---|
| Home (Tasks) | Google Tasks API | tasks |
| Gmail | Gmail API | read, modify and send mail |
| Calendar | Google Calendar API | calendar events, read calendars |
| YouTube | YouTube Data API v3 | YouTube account |
| Drive | Google Drive API | full Drive (to browse all your files) |
| Docs and Sheets | Drive, Docs and Sheets APIs | Docs, Sheets and Drive |

None of these need billing on the Cloud project.

**Downloads** (Settings → Downloads): YouTube downloads use yt-dlp, fetched from its official GitHub releases on first use and checked against the published checksum. Install ffmpeg (`brew install ffmpeg`, `winget install ffmpeg`, `apt install ffmpeg`) for full-quality downloads.

**Appearance** (Settings → Appearance): System (the default, follows your computer), Light or Dark. The sun or moon button at the bottom of the sidebar cycles them; right-click it for a menu. ⌘/Ctrl + Shift + L switches between light and dark. The pop-out player follows along.

## Layout

```
main.go, app.go          Wails setup and the methods the UI calls
pip.go                   the pop-out player: this executable run with --pip, one always-on-top window
internal/auth            loopback + PKCE sign-in, keychain, client JSON / .env, per-feature scopes
internal/db              SQLite open, embedded migrations, settings
internal/google          profile, per-feature API checks
internal/tasks           Google Tasks lists and tasks, cached
internal/gmail           inbox, threads, sending
internal/calendar        calendars, events, repeat rules, cached
internal/youtube         subscriptions feed (RSS, no quota), search, video details
internal/player          loopback player pages (YouTube embed, and the yt-dlp Media Source player)
internal/ytdlp           yt-dlp: downloads, search fallback, stream links, chunked proxy, MP4 index
internal/drive           Drive listing, search, downloads, exports, resumable uploads, cached
internal/docs            Docs rendered to HTML, append, find and replace
internal/sheets          Sheets tabs, cell windows, cell edits, append row
frontend/src/pages       one file per screen
frontend/src/components  reusable pieces; components/ui = Radix + Tailwind primitives
frontend/src/lib/api.js  the only file that calls Go
scripts/                 dev-* to run in dev mode, build-* to make installers
```

Data lives in your OS app-data folder: `~/Library/Application Support/googleplex` on macOS, `%AppData%\googleplex` on Windows, `~/.config/googleplex` on Linux.

## Tests

```bash
go test ./internal/...
```

The Up next queue logic has its own plain-Node tests: `cd frontend && npm test`.

Tests talk to fake Google servers (httptest), never your account. A few opt-in checks hit the network and are skipped unless asked for:
- `GP_LIVE=1` resolves a yt-dlp stream for a Creative Commons test video.
- `GP_SERVE=…` serves the player pages for a browser harness.
