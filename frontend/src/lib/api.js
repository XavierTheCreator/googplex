// The one place the UI talks to Go. Each function is a generated Wails binding.
import * as App from '../../wailsjs/go/main/App'
import * as Pip from '../../wailsjs/go/main/Pip'
import { EventsOn, OnFileDrop, OnFileDropOff } from '../../wailsjs/runtime/runtime'
import { applyTheme } from './theme.js'

export const getSetupState = () => App.GetSetupState()
export const setProjectId = (id) => App.SetProjectID(id)
export const saveClientJson = (text) => App.SaveClientJSON(text)
export const signIn = () => App.SignIn()
export const signOut = () => App.SignOut()
export const connectService = (service) => App.ConnectService(service)
export const checkService = (service) => App.CheckService(service)
export const consoleLinks = () => App.ConsoleLinks()
export const openUrl = (url) => App.OpenURL(url)
export const playerUrl = (videoId) => App.PlayerURL(videoId)

// Wails rejects with a plain string; turn it into a readable message.
export const errorText = (err) => (typeof err === 'string' ? err : err?.message || 'Something went wrong')
export const getYouTubeOverview = () => App.GetYouTubeOverview()
export const getInbox = () => App.GetInbox()
export const openThread = (id) => App.OpenThread(id)
export const getYouTubeFeed = () => App.GetYouTubeFeed()
export const refreshYouTubeFeed = () => App.RefreshYouTubeFeed()
export const sendEmail = (draft) => App.SendEmail(draft)
export const getDownloadSettings = () => App.GetDownloadSettings()
export const chooseDownloadDir = () => App.ChooseDownloadDir()
export const updateYtdlp = () => App.UpdateYtdlp()
export const downloadVideo = (id, title) => App.DownloadVideo(id, title)
export const cancelDownload = (id) => App.CancelDownload(id)
export const revealFile = (path) => App.RevealFile(path)

// Download progress arrives as Wails events; returns the unsubscribe function.
export const onDownloadProgress = (cb) => EventsOn('download:progress', cb)

// Pop-out player window (a second process; see pip.go).
export const popOut = (id, title, start, stream = '') => App.PopOut(id, title, start, stream)
// For videos that block embedding: a loopback page playing a yt-dlp stream.
export const streamUrl = (id) => App.StreamURL(id)
export const bringBack = () => App.BringBack()
export const closePopOut = () => App.ClosePopOut()
export const onPipDocked = (cb) => EventsOn('pip:docked', cb)
export const onPipClosed = (cb) => EventsOn('pip:closed', cb)

// Inside the pop-out window itself.
export const pipConfig = () => Pip.Config()
export const pipPlayerUrl = (id) => Pip.PlayerURL(id)
export const pipReportTime = (t) => Pip.ReportTime(t)
export const pipDock = (t) => Pip.Dock(t)
export const pipClose = () => Pip.Close()
export const searchYouTube = (query, pageToken = '') => App.SearchYouTube(query, pageToken)

// Calendar
export const getCalendars = () => App.GetCalendars()
export const setCalendarVisible = (id, visible) => App.SetCalendarVisible(id, visible)
export const getEvents = (start, end) => App.GetEvents(start.toISOString(), end.toISOString())
export const refreshEvents = (start, end) => App.RefreshEvents(start.toISOString(), end.toISOString())
export const createEvent = (input) => App.CreateEvent(input)
export const updateEvent = (input, scope = 'this') => App.UpdateEvent(scope, input)
export const deleteEvent = (calendarId, eventId, scope = 'this') => App.DeleteEvent(calendarId, eventId, scope)
export const getEvent = (calendarId, eventId) => App.GetEvent(calendarId, eventId)
export const getWeekStart = () => App.GetWeekStart()
export const setWeekStart = (day) => App.SetWeekStart(day)
export const quickAddEvent = (calendarId, text) => App.QuickAddEvent(calendarId, text)

// Drive
export const driveList = (view, folderId = '', orderBy = '', pageToken = '') => App.DriveList(view, folderId, orderBy, pageToken)
export const driveCached = (view, folderId = '') => App.DriveCached(view, folderId)
export const driveSearch = (text, pageToken = '') => App.DriveSearch(text, pageToken)
export const driveGet = (id) => App.DriveGet(id)
export const driveRecent = (kind) => App.DriveRecent(kind)
export const driveExportFormats = (mimeType) => App.DriveExportFormats(mimeType)
export const driveCreateFolder = (parentId, name) => App.DriveCreateFolder(parentId, name)
export const driveRename = (id, name) => App.DriveRename(id, name)
export const driveStar = (id, starred) => App.DriveStar(id, starred)
export const driveTrash = (id) => App.DriveTrash(id)
export const drivePreviewUrl = (id) => App.DrivePreviewURL(id)
export const driveDownload = (id) => App.DriveDownload(id)
export const driveExport = (id, format) => App.DriveExport(id, format)
export const driveUpload = (folderId) => App.DriveUpload(folderId)
export const driveUploadPaths = (folderId, paths) => App.DriveUploadPaths(folderId, paths)
export const driveCancel = (transferId) => App.DriveCancel(transferId)
export const onDriveProgress = (cb) => EventsOn('drive:progress', cb)
// Files dropped on an element styled --wails-drop-target: drop; returns the unsubscribe.
export const onFileDrop = (cb) => {
  OnFileDrop((x, y, paths) => cb(paths, x, y), true)
  return () => OnFileDropOff()
}

// Docs and Sheets
export const docsGet = (id) => App.DocsGet(id)
export const docsAppend = (id, text) => App.DocsAppend(id, text)
export const docsReplace = (id, find, replace, matchCase) => App.DocsReplace(id, find, replace, matchCase)
export const sheetsGet = (id) => App.SheetsGet(id)
export const sheetsValues = (id, sheet, row, count, cols) => App.SheetsValues(id, sheet, row, count, cols)
export const sheetsFormula = (id, sheet, row, col) => App.SheetsFormula(id, sheet, row, col)
export const sheetsUpdate = (id, sheet, row, col, value) => App.SheetsUpdate(id, sheet, row, col, value)
export const sheetsAppend = (id, sheet, cells) => App.SheetsAppend(id, sheet, cells)

// Tasks
export const tasksCached = () => App.TasksCached()
export const tasksLists = () => App.TasksLists()
export const tasksOf = (listId) => App.TasksOf(listId)
export const tasksCachedOf = (listId) => App.TasksCachedOf(listId)
export const tasksCreateList = (title) => App.TasksCreateList(title)
export const tasksRenameList = (id, title) => App.TasksRenameList(id, title)
export const tasksDeleteList = (id) => App.TasksDeleteList(id)
export const tasksCreate = (listId, task) => App.TasksCreate(listId, { title: '', notes: '', due: '', parent: '', previous: '', ...task })
// patch: any of { title, notes, due ('' clears), status: 'needsAction' | 'completed' }
export const tasksUpdate = (listId, taskId, patch) => App.TasksUpdate(listId, taskId, patch)
export const tasksDelete = (listId, taskId) => App.TasksDelete(listId, taskId)
export const tasksMove = (listId, taskId, parent = '', previous = '', destList = '') => App.TasksMove(listId, taskId, parent, previous, destList)
export const tasksClearCompleted = (listId) => App.TasksClearCompleted(listId)

// Gmail
export const gmailList = (q = '', labelIds = [], pageToken = '') => App.GmailList(q, labelIds, pageToken)
export const gmailLabels = () => App.GmailLabels()
export const gmailCreateLabel = (name) => App.GmailCreateLabel(name)
export const gmailRenameLabel = (id, name) => App.GmailRenameLabel(id, name)
export const gmailDeleteLabel = (id) => App.GmailDeleteLabel(id)
// Add and remove labels on threads; messageIds lets several threads change in one batch call.
export const gmailModify = (threadIds, messageIds, add = [], remove = []) => App.GmailModify(threadIds, messageIds, add, remove)
export const gmailTrash = (threadIds, untrash = false) => App.GmailTrash(threadIds, untrash)
export const gmailReplyDraft = (message, mode) => App.GmailReplyDraft(message, mode)
export const gmailSaveDraft = (draft) => App.GmailSaveDraft(draft)
export const gmailDeleteDraft = (id) => App.GmailDeleteDraft(id)
export const gmailDrafts = (pageToken = '') => App.GmailDrafts(pageToken)
export const gmailGetDraft = (id) => App.GmailGetDraft(id)
export const gmailSignature = () => App.GmailSignature()
export const gmailLocalSignature = () => App.GmailLocalSignature()
export const gmailSetLocalSignature = (text) => App.GmailSetLocalSignature(text)
export const gmailContacts = () => App.GmailContacts()
export const gmailPickFiles = () => App.GmailPickFiles()
export const gmailFileInfo = (paths) => App.GmailFileInfo(paths)
export const gmailAttachmentUrl = (messageId, partId) => App.GmailAttachmentURL(messageId, partId)
export const gmailDownloadAttachment = (messageId, partId, name) => App.GmailDownloadAttachment(messageId, partId, name)
export const gmailSaveToDrive = (messageId, partId, name) => App.GmailSaveToDrive(messageId, partId, name)

// YouTube account: playlists, likes, channels, comments
export const ytPlaylists = (channelId = '', pageToken = '') => App.YTPlaylists(channelId, pageToken)
export const ytPlaylistItems = (playlistId, pageToken = '') => App.YTPlaylistItems(playlistId, pageToken)
export const ytCreatePlaylist = (title, privacy = 'private') => App.YTCreatePlaylist(title, privacy)
export const ytRenamePlaylist = (id, title, description = '') => App.YTRenamePlaylist(id, title, description)
export const ytDeletePlaylist = (id) => App.YTDeletePlaylist(id)
export const ytAddToPlaylist = (playlistId, videoId) => App.YTAddToPlaylist(playlistId, videoId)
export const ytRemoveFromPlaylist = (playlistId, itemId) => App.YTRemoveFromPlaylist(playlistId, itemId)
export const ytMovePlaylistItem = (itemId, playlistId, videoId, position) => App.YTMovePlaylistItem(itemId, playlistId, videoId, position)
export const ytMembership = (videoId) => App.YTMembership(videoId)
export const ytLiked = (pageToken = '') => App.YTLiked(pageToken)
export const ytVideo = (id) => App.YTVideo(id)
export const ytRate = (videoId, rating) => App.YTRate(videoId, rating)
export const ytChannel = (id) => App.YTChannel(id)
export const ytSubscription = (channelId) => App.YTSubscription(channelId)
export const ytSubscribe = (channelId) => App.YTSubscribe(channelId)
export const ytUnsubscribe = (channelId, subscriptionId) => App.YTUnsubscribe(channelId, subscriptionId)
export const ytComments = (videoId, order = 'relevance', pageToken = '') => App.YTComments(videoId, order, pageToken)
export const ytReplies = (parentId, pageToken = '') => App.YTReplies(parentId, pageToken)
export const ytPostComment = (videoId, text) => App.YTPostComment(videoId, text)
export const ytReply = (parentId, text) => App.YTReply(parentId, text)
export const ytDeleteComment = (id) => App.YTDeleteComment(id)
export const ytMyChannelId = () => App.YTMyChannelID()
// Kept by the app (YouTube's API has no Watch later or history any more)
export const ytWatchLater = () => App.YTWatchLater()
export const ytAddWatchLater = (video) => App.YTAddWatchLater(video)
export const ytRemoveWatchLater = (videoId) => App.YTRemoveWatchLater(videoId)
export const ytHistory = () => App.YTHistory()
export const ytAddHistory = (video) => App.YTAddHistory(video)
export const ytClearHistory = () => App.YTClearHistory()
export const ytRecentSearches = () => App.YTRecentSearches()
export const ytAddRecentSearch = (q) => App.YTAddRecentSearch(q)
export const ytRemoveRecentSearch = (q) => App.YTRemoveRecentSearch(q)
export const ytClearRecentSearches = () => App.YTClearRecentSearches()
export const onPipEnded = (cb) => EventsOn('pip:ended', cb)
// In the pop-out window: tell the app the video ended.
export const pipEnded = () => Pip.Ended()

// Theme: saved in Go, applied to the page by lib/theme.js
export const getTheme = () => App.GetTheme()
export const setTheme = (theme) => { applyTheme(theme); return App.SetTheme(theme) }
export const onThemeChanged = (cb) => EventsOn('theme:changed', cb) // in the pop-out window
