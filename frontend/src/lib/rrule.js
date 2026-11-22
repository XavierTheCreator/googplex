// Repeat rules for the event form: build RRULE lines, read them back, and
// describe them in words. A rule here is
// { freq: 'DAILY'|'WEEKLY'|'MONTHLY'|'YEARLY', interval, byDay: ['MO', …],
//   monthly: 'day'|'weekday', ends: 'never'|'on'|'after', until: 'YYYY-MM-DD', count }

export const DAYS = ['SU', 'MO', 'TU', 'WE', 'TH', 'FR', 'SA']
const dayNames = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
const ordinals = { 1: 'first', 2: 'second', 3: 'third', 4: 'fourth', '-1': 'last' }

// Which week of the month a date is in: 1-4, or -1 for the last one.
export function nthOfMonth(d) {
  const n = Math.ceil(d.getDate() / 7)
  const next = new Date(d)
  next.setDate(d.getDate() + 7)
  return next.getMonth() !== d.getMonth() && n >= 4 ? -1 : n
}

export function newRule(freq, start) {
  return { freq, interval: 1, byDay: [DAYS[start.getDay()]], monthly: 'day', ends: 'never', until: '', count: 10 }
}

// The quick choices in the Repeat menu, worded for this start date.
export function presets(start) {
  const day = dayNames[start.getDay()]
  const nth = ordinals[nthOfMonth(start)]
  const date = start.toLocaleDateString([], { month: 'long', day: 'numeric' })
  const base = (freq) => newRule(freq, start)
  return [
    { id: 'none', label: 'Does not repeat', rule: null },
    { id: 'daily', label: 'Daily', rule: base('DAILY') },
    { id: 'weekly', label: `Weekly on ${day}`, rule: base('WEEKLY') },
    { id: 'monthlyNth', label: `Monthly on the ${nth} ${day}`, rule: { ...base('MONTHLY'), monthly: 'weekday' } },
    { id: 'monthlyDay', label: `Monthly on day ${start.getDate()}`, rule: base('MONTHLY') },
    { id: 'yearly', label: `Annually on ${date}`, rule: base('YEARLY') },
    { id: 'weekdays', label: 'Every weekday (Monday to Friday)', rule: { ...base('WEEKLY'), byDay: ['MO', 'TU', 'WE', 'TH', 'FR'] } },
    { id: 'custom', label: 'Custom…', rule: null },
  ]
}

// The preset a rule matches, or 'custom'.
export function presetFor(rule, start) {
  if (!rule) return 'none'
  if (rule.raw) return 'custom'
  const line = buildRRULE(rule, start, false)
  return presets(start).find((p) => p.rule && buildRRULE(p.rule, start, false) === line)?.id || 'custom'
}

// RRULE line for a rule starting at `start`. allDay changes how UNTIL is written.
export function buildRRULE(rule, start, allDay) {
  if (rule.raw) return rule.raw
  const parts = [`FREQ=${rule.freq}`]
  if (rule.interval > 1) parts.push(`INTERVAL=${rule.interval}`)
  if (rule.freq === 'WEEKLY' && rule.byDay.length) parts.push(`BYDAY=${DAYS.filter((d) => rule.byDay.includes(d)).join(',')}`)
  if (rule.freq === 'MONTHLY') {
    parts.push(rule.monthly === 'weekday' ? `BYDAY=${nthOfMonth(start)}${DAYS[start.getDay()]}` : `BYMONTHDAY=${start.getDate()}`)
  }
  if (rule.ends === 'after' && rule.count > 0) parts.push(`COUNT=${rule.count}`)
  if (rule.ends === 'on' && rule.until) {
    const [y, m, d] = rule.until.split('-').map(Number)
    if (allDay) parts.push(`UNTIL=${rule.until.replaceAll('-', '')}`)
    else {
      const endOfDay = new Date(y, m - 1, d, 23, 59, 59) // through the end of that day, local time
      parts.push(`UNTIL=${endOfDay.toISOString().replace(/[-:]/g, '').replace(/\.\d{3}/, '')}`)
    }
  }
  return `RRULE:${parts.join(';')}`
}

// Reads the first RRULE from a recurrence list. Rules using parts this form
// can't edit keep `raw`, so saving without changes leaves them exactly as they were.
export function parseRecurrence(lines) {
  const line = (lines || []).find((l) => l.startsWith('RRULE:'))
  if (!line) return null
  const kv = Object.fromEntries(line.slice(6).split(';').map((p) => p.split('=')))
  const rule = { freq: kv.FREQ, interval: Number(kv.INTERVAL || 1), byDay: [], monthly: 'day', ends: 'never', until: '', count: 10 }
  const known = new Set(['FREQ', 'INTERVAL', 'BYDAY', 'BYMONTHDAY', 'COUNT', 'UNTIL', 'WKST'])
  let editable = ['DAILY', 'WEEKLY', 'MONTHLY', 'YEARLY'].includes(rule.freq) && Object.keys(kv).every((k) => known.has(k))
  if (kv.BYDAY) {
    const days = kv.BYDAY.split(',')
    if (rule.freq === 'MONTHLY') {
      rule.monthly = 'weekday'
      if (days.length !== 1 || !/^-?\d[A-Z]{2}$/.test(days[0])) editable = false
    } else if (rule.freq === 'WEEKLY' && days.every((d) => DAYS.includes(d))) {
      rule.byDay = days
    } else {
      editable = false
    }
  }
  if (kv.COUNT) Object.assign(rule, { ends: 'after', count: Number(kv.COUNT) })
  if (kv.UNTIL) {
    const u = kv.UNTIL
    let until = `${u.slice(0, 4)}-${u.slice(4, 6)}-${u.slice(6, 8)}`
    if (/T\d{6}Z$/.test(u)) { // a UTC instant: show the local day it falls on
      const t = new Date(`${until}T${u.slice(9, 11)}:${u.slice(11, 13)}:${u.slice(13, 15)}Z`)
      until = `${t.getFullYear()}-${String(t.getMonth() + 1).padStart(2, '0')}-${String(t.getDate()).padStart(2, '0')}`
    }
    Object.assign(rule, { ends: 'on', until })
  }
  if (!editable) rule.raw = line
  return rule
}

// "Weekly on Tuesday, until Mar 3" and the like.
export function describe(rule, start) {
  if (!rule) return ''
  const n = rule.interval
  const unit = { DAILY: 'day', WEEKLY: 'week', MONTHLY: 'month', YEARLY: 'year' }[rule.freq] || ''
  const every = n > 1 ? `Every ${n} ${unit}s` : { DAILY: 'Daily', WEEKLY: 'Weekly', MONTHLY: 'Monthly', YEARLY: 'Annually' }[rule.freq] || 'Repeats'
  let text = every
  if (rule.freq === 'WEEKLY' && rule.byDay.length) {
    const names = DAYS.filter((d) => rule.byDay.includes(d)).map((d) => dayNames[DAYS.indexOf(d)])
    text += names.join() === 'Monday,Tuesday,Wednesday,Thursday,Friday' ? ' on weekdays' : ` on ${names.join(', ')}`
  }
  if (rule.freq === 'MONTHLY') {
    text += rule.monthly === 'weekday' ? ` on the ${ordinals[nthOfMonth(start)]} ${dayNames[start.getDay()]}` : ` on day ${start.getDate()}`
  }
  if (rule.freq === 'YEARLY') text += ` on ${start.toLocaleDateString([], { month: 'long', day: 'numeric' })}`
  if (rule.ends === 'after') text += `, ${rule.count} times`
  if (rule.ends === 'on' && rule.until) {
    const [y, m, d] = rule.until.split('-').map(Number)
    text += `, until ${new Date(y, m - 1, d).toLocaleDateString([], { month: 'short', day: 'numeric', year: 'numeric' })}`
  }
  return rule.raw ? `${text} (custom rule)` : text
}
