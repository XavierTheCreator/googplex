import Input from './ui/Input.jsx'
import { DAYS, newRule, presetFor, presets } from '../lib/rrule.js'

const select = 'h-9 rounded-md border border-border bg-bg px-2 text-fg focus:outline-none focus:ring-2 focus:ring-accent/50'
const letters = ['S', 'M', 'T', 'W', 'T', 'F', 'S']

// The Repeat menu plus the Custom panel. rule is null for "does not repeat".
export default function RepeatPicker({ rule, start, custom, onChange, onCustom }) {
  const list = presets(start)
  const current = custom ? 'custom' : presetFor(rule, start)

  const pick = (id) => {
    if (id === 'custom') {
      onCustom(true)
      if (!rule) onChange(newRule('WEEKLY', start))
      return
    }
    onCustom(false)
    onChange(list.find((p) => p.id === id).rule)
  }

  const set = (patch) => onChange({ ...rule, ...patch, raw: undefined }) // any edit replaces a rule kept as is

  return (
    <div className="flex-1 space-y-2">
      <select value={current} onChange={(e) => pick(e.target.value)} className={`${select} w-full`}>
        {list.map((p) => <option key={p.id} value={p.id}>{p.label}</option>)}
      </select>
      {current === 'custom' && rule && (
        <div className="space-y-2.5 rounded-md border border-border p-3 text-[13px]">
          {rule.raw && <p className="text-muted">This event uses a repeat rule from Google that this form can't show in full. It stays as is unless you change it here.</p>}
          <div className="flex items-center gap-2">
            Every
            <Input type="number" min={1} max={99} value={rule.interval} onChange={(e) => set({ interval: Math.max(1, Number(e.target.value) || 1) })} className="h-8 w-16" />
            <select value={rule.freq} onChange={(e) => set({ freq: e.target.value })} className={`${select} h-8`}>
              <option value="DAILY">{rule.interval > 1 ? 'days' : 'day'}</option>
              <option value="WEEKLY">{rule.interval > 1 ? 'weeks' : 'week'}</option>
              <option value="MONTHLY">{rule.interval > 1 ? 'months' : 'month'}</option>
              <option value="YEARLY">{rule.interval > 1 ? 'years' : 'year'}</option>
            </select>
          </div>
          {rule.freq === 'WEEKLY' && (
            <div className="flex gap-1">
              {DAYS.map((d, i) => {
                const on = rule.byDay.includes(d)
                return (
                  <button key={d} type="button" aria-pressed={on}
                    onClick={() => set({ byDay: on ? rule.byDay.filter((x) => x !== d) : [...rule.byDay, d] })}
                    className={`grid h-7 w-7 place-items-center rounded-full text-[12px] ${on ? 'bg-accent text-on-accent' : 'bg-raised text-muted hover:text-fg'}`}>
                    {letters[i]}
                  </button>
                )
              })}
            </div>
          )}
          {rule.freq === 'MONTHLY' && (
            <select value={rule.monthly} onChange={(e) => set({ monthly: e.target.value })} className={`${select} h-8`}>
              <option value="day">On day {start.getDate()}</option>
              <option value="weekday">{list.find((p) => p.id === 'monthlyNth').label.replace('Monthly o', 'O')}</option>
            </select>
          )}
          <div className="space-y-1.5">
            <div className="text-muted">Ends</div>
            <label className="flex items-center gap-2"><input type="radio" checked={rule.ends === 'never'} onChange={() => set({ ends: 'never' })} /> Never</label>
            <label className="flex items-center gap-2">
              <input type="radio" checked={rule.ends === 'on'} onChange={() => set({ ends: 'on' })} /> On
              <Input type="date" value={rule.until} onChange={(e) => set({ ends: 'on', until: e.target.value })} className="h-8 w-40" />
            </label>
            <label className="flex items-center gap-2">
              <input type="radio" checked={rule.ends === 'after'} onChange={() => set({ ends: 'after' })} /> After
              <Input type="number" min={1} value={rule.count} onChange={(e) => set({ ends: 'after', count: Math.max(1, Number(e.target.value) || 1) })} className="h-8 w-20" />
              occurrences
            </label>
          </div>
        </div>
      )}
    </div>
  )
}
