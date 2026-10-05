import { useEffect } from 'react';
import { CoderHeader } from './CoderLandingPage';
import { setMeta } from '../meta';

// THE TABLE STAKES, AND WHO ELSE HAS THEM (2026-09-28, Greg: "compare our
// features with top 10 coding agents"). Every "also in" comes from that agent's
// own README or docs, read on 2026-09-28; an agent missing from a row is not
// a claim that it lacks the feature, only that its docs did not say so where
// we read. Memdoor's own gaps are listed too: a page that hides MCP reads as
// evasive to the developers it is for. Rows added after that reading (subagents,
// scheduled checks, approval, sandbox) name only agents whose docs we read for
// them. Copy pass 2026-10-05 with the landing's: each fact once, plainly.
export const SHARED: { feature: string; detail: string; also: string }[] = [
  { feature: 'Works in your terminal', detail: 'memdoor tui, in the directory you launch it from', also: 'Claude Code, Codex CLI, Gemini CLI, Aider, OpenCode, Cline, Copilot CLI, Amp, Goose, Cursor CLI' },
  { feature: 'Reads your project instructions', detail: 'AGENTS.md or CLAUDE.md, only the sections the task needs', also: 'Claude Code, Codex CLI, OpenCode, Amp, Goose (AGENTS.md); Gemini CLI, Cline, Aider (their own files)' },
  { feature: 'Your own key, any model', detail: 'OpenRouter’s catalogue, or Anthropic, OpenAI, Gemini, DeepSeek, Baseten, Groq and xAI on their own keys (memdoor connect); /model shows list prices', also: 'Codex CLI, Cline, Goose, Aider, OpenCode' },
  { feature: 'Work in a git worktree', detail: 'memdoor tui --worktree, your checkout untouched', also: 'Codex CLI' },
  { feature: 'Screenshots as input', detail: 'paste an image (macOS); @ mentions a file or folder', also: 'Codex CLI, Gemini CLI, Aider' },
  { feature: 'Scripts and CI', detail: 'memdoor agent --message, no window needed', also: 'Claude Code, Codex CLI, Gemini CLI, Cline, Goose, Cursor CLI' },
  { feature: 'Memory across sessions', detail: 'per-project notes it reads first and appends to', also: 'Claude Code, Codex CLI' },
  { feature: 'Skills', detail: 'load a procedure by name, or save your own', also: 'Claude Code, Amp' },
  { feature: 'Searches the web', detail: 'web_search answers with sources, on your own OpenRouter key', also: 'Claude Code, Codex CLI, Gemini CLI, omp' },
  { feature: 'Remote control from your phone', detail: '/remote gives a link and a QR code; the page is the terminal itself, end-to-end encrypted', also: 'Claude Code' },
  { feature: 'Share a conversation', detail: '/share gives a read-only link, end-to-end encrypted, secrets removed; /unshare deletes it', also: 'Command Code, Kilo Code, pi, omp' },
  { feature: 'MCP servers', detail: 'paste a URL, a command line or a .mcp.json snippet; OAuth sign-in, trust before a repo\'s servers run', also: 'Claude Code, Codex CLI, Gemini CLI, OpenCode, Cline, Copilot CLI, Amp, Goose, Cursor CLI' },
  { feature: 'Subagents', detail: 'sessions_spawn runs a subtask in its own session, in parallel; its result wakes the conversation that asked', also: 'Claude Code' },
  { feature: 'Scheduled checks', detail: 'the agent schedules its own re-check ("every 20s, 6 times: is CI green?") and answers into the window that asked', also: '' },
  { feature: 'Approval before changes', detail: 'MEMDOOR_APPROVE=changes asks before a command, a write or an MCP tool runs', also: 'Claude Code, Codex CLI, Gemini CLI' },
  { feature: 'Sandbox around commands', detail: 'no, by choice: file tools stay in your project and commands start there', also: 'Claude Code, Codex CLI, Gemini CLI' },
];

// Workflows: the hero's claim ("Don't build code. Build workflows."), listed
// here as facts of pkg/workflow + mario (docs/features/WORKFLOWS.md).
export const WORKFLOWS: { feature: string; detail: string }[] = [
  { feature: 'Steps from a description', detail: 'the coder writes one YAML per step; independent steps run in parallel' },
  { feature: 'Done means the target exists', detail: 'a file exists or a command exits 0; a target already true is skipped' },
  { feature: 'A gate', detail: 'the run waits for your approval; a comment sends the step back' },
  { feature: 'Resume', detail: 'a failed run starts again at the failed step' },
  { feature: 'Schedules', detail: 'memdoor cron add --workflow <name>, on your local cron, free' },
  { feature: 'State on memdoor.ai (Pro)', detail: 'a workflow can wait on what another produced, from any machine' },
];

export const ONLY: { feature: string; detail: string }[] = [
  { feature: 'A model that judges what the agent reads', detail: 'every search hit, file section and log line scored against the task before the coding model pays to read it' },
  { feature: 'A toolbox sized to the turn', detail: 'only the tools this kind of work needs travel with each call' },
  { feature: 'A stop instead of a cap', detail: 'a turn that stops making progress ends with what it has' },
  { feature: '/usage', detail: 'what the decision model kept out of your bill this month' },
];


// FeaturesPage is /features (Greg, 2026-09-28: "no scope creep in landing
// page", "we need a page features with the top features"): the landing stays
// one claim about one coding agent; the full list lives here.
export function FeaturesPage() {
  useEffect(() => {
    setMeta({
      title: 'Features — Memdoor: workflows, judged reads, every coding-agent table stake',
      description:
        'What Memdoor does: workflows with a proof per step and a gate, a decision model that cuts what the agent reads, and every feature the other coding agents document, with who else has it.',
      path: '/features',
    });
  }, []);
  return (
    <div className="flex min-h-screen flex-col bg-white text-neutral-800">
      <CoderHeader />
      <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-14 sm:px-12 sm:py-20">
        <h1 className="mx-auto mb-3 max-w-3xl text-center text-4xl font-bold tracking-tight text-neutral-900 sm:text-5xl">
          Features
        </h1>
        <p className="mx-auto mb-14 max-w-2xl text-center text-base text-neutral-500">
          Workflows, what makes it cheaper, and what every coding agent does, with who else does it.
        </p>

        <h2 className="mb-6 text-center text-2xl font-bold tracking-tight text-neutral-900">Workflows</h2>
        <div className="mx-auto mb-4 grid max-w-5xl grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {WORKFLOWS.map((r) => (
            <div key={r.feature} className="rounded-2xl border-2 border-neutral-900 bg-white p-5">
              <div className="mb-1 font-semibold text-neutral-900">{r.feature}</div>
              <p className="text-sm leading-relaxed text-neutral-500">{r.detail}</p>
            </div>
          ))}
        </div>
        <p className="mx-auto mb-16 max-w-2xl text-center text-sm text-neutral-500">
          <a href="/workflows" className="text-neutral-900 underline underline-offset-4">The catalog</a>, each with a
          recording · <a href="/docs/workflows" className="text-neutral-900 underline underline-offset-4">How they work</a>
        </p>

        <h2 className="mb-6 text-center text-2xl font-bold tracking-tight text-neutral-900">What makes it cheaper</h2>
        <div className="mx-auto mb-16 grid max-w-5xl grid-cols-1 gap-4 sm:grid-cols-2">
          {ONLY.map((r) => (
            <div key={r.feature} className="rounded-2xl border-2 border-neutral-900 bg-white p-5">
              <div className="mb-1 font-semibold text-neutral-900">{r.feature}</div>
              <p className="text-sm leading-relaxed text-neutral-500">{r.detail}</p>
            </div>
          ))}
        </div>

        {/* Remote control has a block of its own (Greg, 2026-09-29: "the /remote
            is a killer feature", "let's add /remote in features page"). */}
        <h2 className="mb-6 text-center text-2xl font-bold tracking-tight text-neutral-900">
          The same terminal, on your phone
        </h2>
        <div className="mx-auto mb-16 max-w-5xl rounded-2xl bg-neutral-950 p-6 text-white sm:p-8">
          <p className="mb-6 max-w-3xl text-sm leading-relaxed text-neutral-300">
            Type <code className="rounded bg-neutral-800 px-1.5 py-0.5 text-white">/remote</code> and scan the code.
            The page that opens is your terminal, drawn on your phone, end-to-end encrypted.{' '}
            <code className="text-white">/remote view</code> gives a link that watches and cannot type.
          </p>
          <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-sm">
            <a href="/#remote" className="font-semibold text-white underline underline-offset-4">
              See it on a phone
            </a>
            <a href="/docs/remote" className="text-neutral-300 underline underline-offset-4 hover:text-white">
              How it works
            </a>
            <a href="/pricing" className="text-neutral-500 underline underline-offset-4 hover:text-white">
              Pro, $10 a month
            </a>
          </div>
        </div>

        <h2 className="mb-3 text-center text-2xl font-bold tracking-tight text-neutral-900">
          Everything you expect from a coding agent
        </h2>
        <p className="mx-auto mb-8 max-w-2xl text-center text-sm text-neutral-500">
          Compared with Claude Code, Codex CLI, Gemini CLI, Aider, OpenCode, Cline, Copilot CLI, Amp, Goose and Cursor
          CLI (and, for sharing, Command Code, Kilo Code, pi and omp), from their own docs. The right-hand column
          says who else documents each one.
        </p>
        <div className="mx-auto max-w-5xl overflow-x-auto">
          <table className="w-full min-w-[40rem] border-collapse text-left text-sm">
            <thead>
              <tr className="border-b border-neutral-200 text-xs uppercase tracking-wider text-neutral-400">
                <th className="py-3 pr-4 font-medium">Feature</th>
                <th className="py-3 pr-4 font-medium">In Memdoor</th>
                <th className="py-3 font-medium">Also in</th>
              </tr>
            </thead>
            <tbody>
              {SHARED.map((r) => (
                <tr key={r.feature} className="border-b border-neutral-100 align-top">
                  <td className="py-3 pr-4 font-medium text-neutral-900">{r.feature}</td>
                  <td className="py-3 pr-4 text-neutral-600">{r.detail}</td>
                  <td className="py-3 text-neutral-500">{r.also}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="mx-auto mt-8 max-w-2xl text-center text-xs text-neutral-400">
          Read from each agent’s docs on 28 September 2026; an agent not named in a row may still have the feature.
        </p>
        <div className="mt-12 text-center">
          <a href="/#install" className="inline-block rounded-full bg-neutral-900 px-6 py-3 text-sm font-semibold text-white hover:bg-neutral-700">
            Install it
          </a>
        </div>
      </main>
    </div>
  );
}
