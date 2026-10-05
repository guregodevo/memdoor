import { useEffect } from 'react';
import { CoderHeader } from './CoderLandingPage';
import { Cast } from './Cast';
import { setMeta } from '../meta';

// THE WORKFLOW CATALOG (Greg, 2026-10-04: "instead of Features let's write
// Workflows", "like a catalog of workflows"). Every entry is a workflow that
// exists as files and has run; the steps column is its real graph, read from
// its task files. A cast is a real session of that workflow, never a mock —
// and an entry WITHOUT a cast is not listed (Greg, 2026-10-05: "remove those
// that don't have demo").
type Entry = {
  name: string;
  kind: string;
  what: string;
  steps: string;
  gate?: string;
  run: string;
  cast?: { src: string; poster: string };
  source?: { label: string; href: string };
};

const CATALOG: Entry[] = [
  {
    name: 'ship',
    kind: 'This repo',
    what: 'How Memdoor itself is released: build, vet, the Go suite, the unused-code check and the web type check in parallel, a red one blocking the rest; push; your yes; deploy; then memdoor.ai read back — the served version, the health routes, a screenshot of the home page. It never commits: it ships what is committed.',
    steps: 'clean → build + vet + tests + unused + web → push → approve → deploy → verify → page',
    gate: 'Nothing is deployed until you approve.',
    run: 'memdoor workflow run ship --partition $(git rev-parse --short HEAD)',
    cast: { src: '/workflow-ship.cast', poster: 'npt:0:14' },
  },
  {
    name: 'review',
    kind: 'This repo',
    what: 'The review loop on one commit: findings cited file:line and verified by running, the untouched files swept for the same class of claim, written to a review you read at the gate; then the fixes applied and committed, and vet plus the suite on the result. Its first run found a real defect in a commit shipped an hour before.',
    steps: 'review → approve → fix → check',
    gate: 'You read the review before anything is changed.',
    run: 'memdoor workflow run review --partition <commit>',
    cast: { src: '/workflow-review.cast', poster: 'npt:0:48' },
  },
  {
    name: 'tui-check',
    kind: 'This repo',
    what: 'The real terminal, tested: a tmux pane runs memdoor tui against the gateway, types a prompt in a scratch project, keeps the screen, and the assertion is on the file the turn made — never on the echoed prompt, which is how its own first run passed falsely.',
    steps: 'tui → assert',
    run: 'memdoor workflow run tui-check',
    cast: { src: '/workflow-tui-check.cast', poster: 'npt:0:25' },
  },
  {
    name: 'issue-to-pr',
    kind: 'Code',
    what: 'Reads a GitHub issue, reproduces it with a failing test, fixes it, writes the PR, and opens it after your review. One comment at the gate sends the fix back.',
    steps: 'issue + branch → reproduce → fix → pr → review → open',
    gate: 'You review the fix and the diff before anything is pushed.',
    run: 'memdoor workflow run issue-to-pr --partition <issue number>',
    cast: { src: '/review-loop.cast', poster: 'npt:0:31' },
  },
  {
    name: 'release-check',
    kind: 'Code',
    what: 'Vet and tests in parallel, release notes from the recent commits, then waits for your approval before announcing.',
    steps: 'vet + tests → changelog → approve → announce',
    gate: 'Nothing is announced until you approve.',
    run: 'memdoor workflow run release-check',
    cast: { src: '/workflow-release.cast', poster: 'npt:0:31' },
  },
  {
    name: 'digest',
    kind: 'Code',
    what: "The project's last ten commits, summarized in three bullets and written to DIGEST.md.",
    steps: 'log → summary → digest',
    run: 'memdoor workflow run digest',
    cast: { src: '/workflow-digest.cast', poster: 'npt:0:30' },
  },
  {
    name: 'karpathy-recipe',
    kind: 'From an article',
    what: 'Karpathy\'s recipe for training neural networks, on CIFAR-10: the data, a skeleton whose loss at init must be −log(10), baselines, an 8-image overfit, then plans for the GPU run. Built by article-to-workflow.',
    steps: 'fetch → data → skeleton → baselines → regularize → tune · plans → approve → squeeze',
    gate: 'You read the GPU and data plans and approve the spend.',
    run: 'memdoor workflow run karpathy-recipe',
    cast: { src: '/workflow-karpathy.cast', poster: 'npt:0:31' },
    source: { label: 'A Recipe for Training Neural Networks', href: 'https://karpathy.github.io/2019/04/25/recipe/' },
  },
];

export function WorkflowCatalogPage() {
  useEffect(() => {
    setMeta({
      title: 'Workflows — a catalog of agent workflows that have run, each with its real session',
      description:
        'Every entry is a workflow that exists as task files and has run: ship, review, tui-check, issue-to-pr, release-check, digest, karpathy-recipe. A graph of steps with a proof per step and a gate where a person decides, recorded live.',
      path: '/workflows',
    });
  }, []);
  return (
    <div className="flex min-h-screen flex-col bg-white text-neutral-800">
      <div className="bg-neutral-950 text-white">
        <CoderHeader dark />
        <section className="mx-auto w-full max-w-6xl px-6 pb-14 pt-10 sm:px-12">
          <div className="mb-3 text-xs uppercase tracking-[0.2em] text-neutral-400">Workflows</div>
          <h1 className="mb-4 text-4xl font-bold tracking-tight sm:text-5xl">Don't build code. Build workflows.</h1>
          <p className="max-w-2xl text-lg text-neutral-300">
            Each one below is a folder of task files that has run for real: a graph of steps, each done only when
            its proof holds, and a gate where a person decides.
          </p>
        </section>
      </div>
      <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-14 sm:px-12">
        <div className="grid grid-cols-1 gap-8">
          {CATALOG.map((w) => (
            <article key={w.name} id={w.name} className="scroll-mt-4 rounded-2xl border border-neutral-200 p-6 sm:p-8">
              <div className="mb-2 flex flex-wrap items-baseline gap-3">
                <h2 className="font-mono text-xl font-semibold text-neutral-900">{w.name}</h2>
                <span className="rounded-full bg-neutral-100 px-2.5 py-0.5 text-xs text-neutral-500">{w.kind}</span>
              </div>
              <p className="mb-4 max-w-3xl text-sm leading-relaxed text-neutral-600">{w.what}</p>
              <dl className="mb-4 grid max-w-3xl grid-cols-[5rem_1fr] gap-x-4 gap-y-2 text-sm">
                <dt className="text-neutral-400">Steps</dt>
                <dd className="font-mono text-[13px] text-neutral-800">{w.steps}</dd>
                {w.gate && (
                  <>
                    <dt className="text-neutral-400">Gate</dt>
                    <dd className="text-neutral-700">{w.gate}</dd>
                  </>
                )}
                {w.source && (
                  <>
                    <dt className="text-neutral-400">Source</dt>
                    <dd>
                      <a href={w.source.href} className="text-neutral-700 underline underline-offset-4 hover:text-neutral-900">
                        {w.source.label}
                      </a>
                    </dd>
                  </>
                )}
                <dt className="text-neutral-400">Run</dt>
                <dd>
                  <code className="rounded bg-neutral-100 px-2 py-1 text-[13px] text-neutral-800">{w.run}</code>
                </dd>
              </dl>
              {w.cast && (
                <div className="mt-6 max-w-3xl">
                  <Cast src={w.cast.src} poster={w.cast.poster} />
                </div>
              )}
            </article>
          ))}
        </div>
        <p className="mt-12 text-center text-sm text-neutral-500">
          How they work, and how to write your own:{' '}
          <a href="/docs/workflows" className="text-neutral-800 underline underline-offset-4">the workflows guide</a>.
        </p>
      </main>
    </div>
  );
}
