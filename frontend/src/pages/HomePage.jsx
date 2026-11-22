import { useState } from 'react'
import { CalendarDays } from 'lucide-react'
import PageHeader from '../components/PageHeader.jsx'
import FeatureGate from '../components/FeatureGate.jsx'
import Button from '../components/ui/Button.jsx'
import TasksPanel from '../components/TasksPanel.jsx'
import TodayEvents from '../components/TodayEvents.jsx'

// Home is Google Tasks, with today's calendar beside it once Calendar is set
// up. On narrow windows the calendar column folds away behind a button.
export default function HomePage({ account, onNavigate }) {
  const first = account?.name?.split(' ')[0]
  const [showToday, setShowToday] = useState(false)
  const today = new Date().toLocaleDateString([], { weekday: 'long', month: 'long', day: 'numeric' })

  return (
    <div className="flex h-full flex-col">
      <PageHeader title={first ? `Hello, ${first}` : 'Home'} subtitle={today}>
        <Button size="sm" variant="ghost" className="xl:hidden" onClick={() => setShowToday((s) => !s)}>
          <CalendarDays size={14} /> {showToday ? 'Hide today' : 'Today'}
        </Button>
      </PageHeader>
      <div className="flex min-h-0 flex-1">
        <div className="flex min-w-0 flex-1 flex-col">
          <FeatureGate service="tasks" apiName="Google Tasks API" why="Your to-do lists come from Google Tasks.">
            <TasksPanel />
          </FeatureGate>
        </div>
        <aside className={`${showToday ? 'block' : 'hidden'} w-80 shrink-0 overflow-y-auto border-l border-border p-4 xl:block`}>
          <TodayEvents onOpen={() => onNavigate('calendar')} />
        </aside>
      </div>
    </div>
  )
}
