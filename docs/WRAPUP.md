# Googleplex wrap-up

What the rebuild does, where its limits are, how to build it, and what's left.

## Built, by section

**Setup and sign-in**
- Step-by-step Cloud setup with deep links into your own project.
- Loopback + PKCE sign-in, with secrets in the system keychain.
- Each feature checks its own APIs and asks for its own permission the first time you open it (incremental consent).

**Home (Google Tasks)**
- Task lists with open counts, plus new, rename and delete (Google won't let you delete the default list).
- Quick add (press N) and subtasks.
- Drag to reorder among siblings.
- Round checkboxes, with a collapsible Completed section and Clear completed.
- Inline title editing. A details dialog for notes, due date (with Today, Tomorrow and Next week) and moving to another list.
- Due badges (Overdue, Today, Tomorrow, date) and filters: All, Today, Upcoming, Overdue.
- Delete with a 5-second Undo.
- Keyboard: N, Space, Delete, E.
- Changes show at once and roll back with a message if Google refuses.
- Today's calendar sits in a right-hand column.

**Gmail**
- Folders: Inbox (with Primary, Social, Promotions, Updates and Forums tabs), Starred, Important, Sent, Drafts, All Mail, Spam and Trash.
- Your labels, nested by "/", with their colors and unread counts. Create, rename and delete labels, and apply or remove them on conversations.
- Search with Gmail's own syntax, plus quick chips: Unread, Has attachment, From me, Last 7 days. Infinite scroll, with the inbox cached for instant load.
- Actions on one conversation or many (checkboxes, shift-click for a range): archive, trash and restore, spam and not spam, star, read and unread, important, move to a label. Undo for archive, trash, spam and move.
- Gmail keyboard shortcuts: j/k, o or Enter, e, #, s, shift-U, r, a, f, c, /, x.
- Compose:
  - Reply, Reply all (never to yourself), and Forward with the original attachments.
  - Cc and Bcc, with address chips suggested from people you've mailed.
  - A rich-text editor: bold, italic, underline, links, lists, quote.
  - Your Gmail signature, or a local one from Settings.
  - Attachments by picker or drag and drop, with the 25 MB limit enforced.
- Drafts save to Gmail as you type. The Drafts folder lists them; open one to continue, or discard it.
- Attachments: preview images and PDFs, download, or Save to Drive. Inline (cid:) images show in place.
- Thread view: older messages collapsed, headers on demand, Print and Show original (in Gmail, in the browser).
- Emails render sanitized under a strict content policy:
  - Web links open in the browser, and mailto opens in-app compose.
  - GET forms open in the browser; other forms point you to Gmail.
  - Hovering a link shows its target.
- Mail is never permanently deleted, only moved to the trash.

**Calendar**
- Week, Month and Agenda views, a calendar list with colors, and quick add.
- Event details with RSVP states, Join meeting, and an Open in Google link.
- Create and edit events with guests and repeat rules (presets and Custom).
- This / following / all choices for repeating events.
- Drag to move and resize, and moving between calendars.
- Week starts Sunday or Monday (Settings).
- A Today card.

**YouTube**
- Tabs: Feed (subscriptions via RSS, no quota), Search (API, falling back to yt-dlp), Playlists, Liked, Watch later, History, Subscriptions, Your channel.
- Playlists: yours, with create, rename and delete. Open one to Play all, Shuffle, drag to reorder, or remove videos. Save to playlist from any video's ⋯ menu, with checkboxes like YouTube's.
- Liked videos come from YouTube. Watch later and History are kept by the app, because YouTube's API no longer offers them; the app says so on those tabs. Clear history is on the History tab.
- Under the player:
  - Like and Dislike with your current rating, and Subscribe or Unsubscribe.
  - The description, with links and timestamps that jump the player.
  - Comments (Top or Newest, Load more, replies): post, reply, and delete your own.
- Channel pages in the app: banner, avatar, subscribers, Subscribe, videos (from the uploads playlist, never search) and public playlists.
- Up next queue: Play next and Add to queue on any video, drag to reorder, and autoplay. It works in the embedded player, the yt-dlp player and the pop-out.
- Player keys: k or space, j/l, f, shift-N.
- Recent searches as chips.
- The embedded player plus a pop-out always-on-top window, which docks back at the same time and remembers its position.
- Videos that block embedding play through yt-dlp in a single Media Source player.
- Downloads with progress.

**Drive**
- My Drive, Shared, Recent, Starred and Trash.
- Breadcrumbs, search, list and grid, sortable columns, and a right-click menu.
- Download, and export of Google files (DOCX, XLSX, PPTX, PDF…).
- Resumable uploads (button or drag and drop) with progress.
- New folder, rename, star, and move to trash (never a permanent delete, never sharing changes).
- Image and PDF previews. Listings cached.

**Docs and Sheets**
- Docs render read-only (headings, formatting, links, lists, tables, images), with Append, Find and replace, and Edit title.
- Sheets show as a grid with frozen rows and tabs. Cell edits accept formulas, with Undo of the last edit and Add row.
- Edits only when Drive says you can edit the file.

**Appearance**
- Light, Dark or System (default). Settings card, sidebar button (click cycles, right-click for a menu) and ⌘/Ctrl + Shift + L.
- The choice is saved locally and applied before the window opens, so there's no flash; the macOS title bar and the pop-out follow it.
- Colors meet WCAG AA contrast (4.5:1) in both themes; text on calendar colors picks black or white by contrast.
- Plain-text emails follow the theme. HTML emails stay on a white card as the sender designed them; in dark mode each message has a **Dark email** switch (off by default) that inverts the email but keeps pictures in their real colors.

## Known limits

**Tasks**
- Due dates are dates only: Google Tasks stores no time of day.
- Drag reorders within the same parent. It doesn't turn a task into a subtask.
- Open counts for lists you haven't opened this session come from the cache.

**Gmail**
- Snoozed isn't in Gmail's API, so there's no Snoozed folder.
- Search runs within the folder you're in; choose All Mail to search everything.
- Draft autosave re-uploads attachments each time it saves.
- "Show original" and "Print" open Gmail in the browser.
- Replies quote the original as plain text, so its formatting isn't copied.

**Calendar**
- Resize only in Week view, by the bottom edge.
- Weeks start Sunday or Monday only.
- "This and following" on a series with a set number of occurrences gives the new part no end.

**YouTube**
- Search allows 100 API searches a day per Cloud project (Google's default), then falls back to yt-dlp, which has no Load more and no upload dates.
- Quota is 10,000 units a day. Reads cost 1 unit and are cached a few minutes. Writes (like, subscribe, comment, playlist changes) cost about 50 each.
- Watch later and watch history on YouTube itself can't be read by apps. The app's own lists don't sync with YouTube.
- "Save to playlist" checks each of your playlists for the video, which costs 1 unit per playlist.
- Subscription cards open the channel page; unsubscribe is on the channel page.
- Age-restricted videos may not play or download, because yt-dlp isn't signed in.
- yt-dlp playback is capped at 1080p.
- YouTube removed the combined 360p file, so there's no lower-quality fallback.
- Without ffmpeg, downloads are single files, usually up to 720p.
- Autoplay in the pop-out reopens the window for the next video.

**Pop-out player**
- About a second to reload when moving between windows, and up to 2 seconds off on resume.
- In `wails dev` it shows the last built frontend.

**Drive**
- Files only (no folder uploads).
- Google won't export files over 10 MB.
- Trash is view-only here (restore it in Drive).

**Docs and Sheets**
- No full rich-text editor (append and replace only).
- Sheets show columns A–Z, loading 200 rows at a time.

**Testing**
- Everything was built and tested on macOS (Apple Silicon).
- Windows and Linux are compile-checked (the Windows exe cross-compiles) but not run.

## Building installers

- **macOS:** `./scripts/build-mac.sh` gives `Googleplex.app` and `Googleplex-mac-arm64.dmg`, signed ad hoc. Notarization is optional.
- **Windows:** `.\scripts\build-windows.ps1` gives `googleplex.exe`, plus `Googleplex-amd64-installer.exe` when NSIS is installed. The installer handles WebView2.
- **Linux:** `./scripts/build-linux.sh` gives `googleplex`, plus a `.deb` when `dpkg-deb` exists. It needs `libgtk-3-dev libwebkit2gtk-4.1-dev`.

Details are in the README.

## Left from the roadmap

- **Gemini** (AI features with your own API key): not started.
- **Google Play library, Google News:** not started. Neither has a usable personal API, so they would need another approach.
- **Places and Maps:** cancelled. They need billing on the Cloud project. Since March 2025 Google gives free calls per SKU each month instead of a $200 credit: 1,000 for the Enterprise SKUs (search with ratings and hours, details with reviews, photos) and 10,000 for map loads and autocomplete.
- **A Git repository, GitHub, CI** (build and test on all three OSes), **and release builds:** not set up. The project isn't under version control yet.
- **Running it on Windows and Linux machines:** needed before sharing it, especially the sign-in flow, the keychain, file drop and the pop-out window.
- **Code signing and notarization:** for sharing beyond your own machines.
