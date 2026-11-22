import { useEffect, useRef, useState } from 'react'
import { Loader2, Upload } from 'lucide-react'
import SetupStep from '../components/SetupStep.jsx'
import ExternalLink from '../components/ExternalLink.jsx'
import Button from '../components/ui/Button.jsx'
import Input from '../components/ui/Input.jsx'
import { consoleLinks, errorText, getSetupState, saveClientJson, setProjectId, signIn } from '../lib/api.js'

// First launch: only what sign-in needs. Each feature sets up its own API later.
export default function SetupPage({ state, onChange }) {
  const [links, setLinks] = useState({})
  const [project, setProject] = useState(state.projectId || '')
  const [projectSaved, setProjectSaved] = useState(!!state.projectId)
  const [brandingDone, setBrandingDone] = useState(state.hasClient)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const fileInput = useRef(null)

  useEffect(() => { consoleLinks().then(setLinks) }, [state.projectId])

  const saveProject = async () => {
    await setProjectId(project)
    setProjectSaved(true)
    onChange(await getSetupState())
  }

  const readClientFile = async (file) => {
    setError('')
    try {
      onChange(await saveClientJson(await file.text()))
    } catch (err) {
      setError(errorText(err))
    }
  }

  const doSignIn = async () => {
    setBusy(true)
    setError('')
    try {
      onChange(await signIn())
    } catch (err) {
      setError(errorText(err))
    } finally {
      setBusy(false)
    }
  }

  const step = state.hasClient ? 4 : !projectSaved ? 1 : !brandingDone ? 2 : 3

  return (
    <div className="h-full overflow-y-auto">
      <div className="drag h-10" />
      <div className="mx-auto max-w-2xl px-8 pb-16 pt-6">
        <h1 className="text-[24px] font-semibold tracking-tight">Set up Googleplex</h1>
        <p className="mt-2 text-muted">
          Googleplex talks to Google through your own Cloud project, so your data never passes through anyone else.
          This takes about three minutes. Gmail, Calendar and YouTube each ask for their own small step the first time you open them.
        </p>

        <ol className="mt-8 space-y-3">
          <SetupStep number={1} title="Create or pick a Google Cloud project" active={step === 1} done={step > 1}>
            <p>Create a project (any name, for example “Googleplex”), then paste its project ID here so the next links open straight into it.</p>
            <div className="flex gap-2">
              <ExternalLink href={links.createProject}>Create project</ExternalLink>
            </div>
            <div className="flex gap-2">
              <Input placeholder="Project ID, e.g. my-project-123456" value={project} onChange={(e) => { setProject(e.target.value); setProjectSaved(false) }} />
              <Button variant="primary" onClick={saveProject} disabled={!project.trim()}>Save</Button>
            </div>
          </SetupStep>

          <SetupStep number={2} title="Set up the Google sign-in screen" active={step === 2} done={step > 2}>
            <p>
              Open Google Auth Platform and press <span className="text-fg">Get started</span>. Use “Googleplex” as the app name and your own email
              for support. For audience, choose <span className="text-fg">Personal</span> if it is offered. Otherwise choose
              <span className="text-fg"> External</span> and add your own email under Test users.
            </p>
            <div className="flex flex-wrap gap-2">
              <ExternalLink href={links.authOverview}>Open Google Auth Platform</ExternalLink>
              <ExternalLink href={links.audience} variant="ghost">Audience and test users</ExternalLink>
            </div>
            <Button variant="primary" onClick={() => setBrandingDone(true)}>I've done this</Button>
          </SetupStep>

          <SetupStep number={3} title="Create a Desktop app client" active={step === 3} done={step > 3}>
            <p>
              Create an OAuth client with application type <span className="text-fg">Desktop app</span>, then download its JSON file
              and drop it here. The secret is saved in your system keychain.
            </p>
            {state.clientSource === 'env' && <p className="text-success">Using the client from your .env file (dev build).</p>}
            <div className="flex flex-wrap gap-2">
              <ExternalLink href={links.createClient}>Create OAuth client</ExternalLink>
              <Button variant="primary" onClick={() => fileInput.current?.click()}>
                <Upload size={14} /> Choose JSON file
              </Button>
              <input ref={fileInput} type="file" accept=".json,application/json" className="hidden"
                onChange={(e) => e.target.files?.[0] && readClientFile(e.target.files[0])} />
            </div>
          </SetupStep>

          <SetupStep number={4} title="Sign in with Google" active={step === 4} done={state.signedIn}>
            <p>Your browser opens Google's sign-in page. Googleplex only asks for your name and email now.</p>
            <Button variant="primary" onClick={doSignIn} disabled={busy}>
              {busy && <Loader2 className="animate-spin" size={14} />} {busy ? 'Waiting for your browser…' : 'Sign in with Google'}
            </Button>
          </SetupStep>
        </ol>

        {error && <p className="selectable mt-4 rounded-md border border-danger/30 bg-danger/10 p-3 text-danger">{error}</p>}
      </div>
    </div>
  )
}
