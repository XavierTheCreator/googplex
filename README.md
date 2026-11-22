# Googleplex

A desktop app that puts your Google services in one window.

## What it does

- **Tasks** on the home screen: lists, subtasks, due dates
- **Gmail**: read, search, label, reply and send
- **Calendar**: week, month and agenda views; create, drag and repeat events
- **YouTube**: watch, search, a pop-out player you can drag anywhere, and downloads
- **Drive**: browse, search, upload, download and preview
- **Docs and Sheets**: read documents, edit cells
- **Light and dark mode**, or follow your computer's setting

## Download

| System | File |
|---|---|
| macOS (Apple Silicon) | [Googleplex-mac-arm64.dmg](builds/Googleplex-mac-arm64.dmg) |
| Windows 10/11 (64-bit) | [Googleplex-windows-amd64.exe](builds/Googleplex-windows-amd64.exe) |
| Linux | Build from source (below) |

The builds aren't signed, so the first time you open them:

- **macOS:** right-click the app and choose **Open**, or go to System Settings → Privacy & Security → **Open Anyway**.
- **Windows:** when SmartScreen appears, click **More info** → **Run anyway**.

## What you need

- A Google account.
- Your own free Google Cloud project. The app walks you through making one the first time you open it (about 5 minutes). No billing needed.

Your data only travels between your computer and Google.

## Quick start

1. Download the app for your system and open it.
2. Follow the setup screen: create a Google Cloud project, create an OAuth client, and sign in.
3. Open any section. The first time, it asks you to turn on that Google API and allow access.

That's it.

## Build from source

You need Go 1.26+, Node 20+ and the [Wails v2](https://wails.io) CLI (the scripts install Wails if it's missing).

```bash
./scripts/build-mac.sh        # macOS
.\scripts\build-windows.ps1   # Windows
./scripts/build-linux.sh      # Linux
```

Builds land in `build/bin/`. See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for details, dev mode and tests, and [docs/WRAPUP.md](docs/WRAPUP.md) for everything each section does.

## Privacy

Sign-in tokens are stored in your system keychain, and everything else is cached locally in a SQLite file. There are no Googleplex servers; the app talks only to Google.

## License

License: TBD
