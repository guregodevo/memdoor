import { useEffect } from 'react';
import { CoderHeader, InstallLine } from './CoderLandingPage';
import { ONLY, SHARED, WORKFLOWS } from './FeaturesPage';
import { setMeta } from '../meta';

// /compare/<agent>: one page per coding agent people search against
// ("claude code alternative open source"). The rows are the /features table —
// a feature is credited to the other agent only where its own docs said so on
// 2026-09-28 — and the paragraphs say what each is for. Nothing here is a
// claim about the other agent beyond its documented features.
type Rival = {
  slug: string;
  name: string;      // as it appears in the /features "also in" column
  maker: string;
  what: string;      // one sentence on what it is
  pick: string;      // when to pick it over Memdoor
  pickMemdoor: string;
};

export const RIVALS: Rival[] = [
  {
    slug: 'claude-code',
    name: 'Claude Code',
    maker: 'Anthropic',
    what: 'Anthropic’s coding agent for the terminal, on Claude models through a Claude subscription or an Anthropic API key.',
    pick: 'You are on a Claude plan and want the agent built for those models, with a sandbox and a large ecosystem of hooks and integrations.',
    pickMemdoor: 'You want the same kind of agent on any provider’s key (Anthropic’s included), open source, with workflows that have a target per step and a gate for you, and a decision model that cuts what the model reads.',
  },
  {
    slug: 'codex-cli',
    name: 'Codex CLI',
    maker: 'OpenAI',
    what: 'OpenAI’s open-source coding agent for the terminal, on OpenAI models, with a sandbox around commands.',
    pick: 'You are on OpenAI models and want OpenAI’s own agent, sandboxed, with approval modes.',
    pickMemdoor: 'You want one agent across providers, workflows run as a graph with proof per step, and a decision model in front of the chat model.',
  },
  {
    slug: 'gemini-cli',
    name: 'Gemini CLI',
    maker: 'Google',
    what: 'Google’s open-source coding agent for the terminal, on Gemini models, with a sandbox around commands.',
    pick: 'You are on Gemini and want Google’s agent with its sandbox and free tier.',
    pickMemdoor: 'You want Gemini (or any other model) inside workflows with targets, gates and resume, and judged reads on your key.',
  },
  {
    slug: 'aider',
    name: 'Aider',
    maker: 'open source',
    what: 'A git-centred pair programmer in the terminal that works with many models; edits land as commits.',
    pick: 'You want a mature, git-first editing loop and a large community.',
    pickMemdoor: 'You want multi-step work run as a workflow (parallel steps, a gate, resume), remote control from a phone, and the decision model’s savings.',
  },
  {
    slug: 'pi',
    name: 'pi',
    maker: 'open source',
    what: 'A deliberately small coding agent: a short loop, a few tools, extensions for the rest.',
    pick: 'You want the smallest possible agent and will write the orchestration around it yourself.',
    pickMemdoor: 'You want the orchestration built in: workflows with proof per step, gates, schedules, and a decision model, without assembling them.',
  },
];

function has(also: string, name: string) {
  return also.split(/[;,]\s*|\s\(/).some((part) => part.trim().replace(/\)$/, '') === name || part.trim().startsWith(name + ' '));
}

export function ComparePage({ slug }: { slug: string }) {
  const rival = RIVALS.find((r) => r.slug === slug);
  useEffect(() => {
    if (!rival) return;
    setMeta({
      title: `Memdoor vs ${rival.name}: an open-source alternative on your own API key`,
      description: `${rival.name} and Memdoor side by side: the features both document, what only Memdoor has (workflows with a target per step, a decision model), and when to pick each.`,
      path: `/compare/${rival.slug}`,
    });
  }, [rival]);
  if (!rival) {
    return (
      <div className="flex min-h-screen flex-col bg-white text-neutral-800">
        <CoderHeader />
        <main className="mx-auto w-full max-w-3xl flex-1 px-6 py-20 text-center">
          <h1 className="mb-4 text-3xl font-bold text-neutral-900">No such comparison</h1>
          <p className="text-neutral-500">
            {RIVALS.map((r, i) => (
              <span key={r.slug}>
                <a className="underline underline-offset-4" href={`/compare/${r.slug}`}>Memdoor vs {r.name}</a>
                {i < RIVALS.length - 1 ? ' · ' : ''}
              </span>
            ))}
          </p>
        </main>
      </div>
    );
  }
  const both = SHARED.filter((r) => has(r.also, rival.name));
  const onlyMemdoor = SHARED.filter((r) => !has(r.also, rival.name) && !r.detail.startsWith('no,'));
  return (
    <div className="flex min-h-screen flex-col bg-white text-neutral-800">
      <CoderHeader />
      <main className="mx-auto w-full max-w-5xl flex-1 px-6 py-14 sm:px-12 sm:py-20">
        <div className="mb-3 text-center text-xs uppercase tracking-widest text-neutral-500">Compared</div>
        <h1 className="mx-auto mb-4 max-w-3xl text-center text-4xl font-bold tracking-tight text-neutral-900 sm:text-5xl">
          Memdoor vs {rival.name}
        </h1>
        <p className="mx-auto mb-12 max-w-2xl text-center text-base text-neutral-500">{rival.what}</p>

        <div className="mb-14 grid grid-cols-1 gap-6 sm:grid-cols-2">
          <div className="rounded-2xl border border-neutral-200 p-6">
            <h2 className="mb-2 text-lg font-semibold text-neutral-900">Pick {rival.name} when</h2>
            <p className="text-sm leading-relaxed text-neutral-600">{rival.pick}</p>
          </div>
          <div className="rounded-2xl border-2 border-neutral-900 p-6">
            <h2 className="mb-2 text-lg font-semibold text-neutral-900">Pick Memdoor when</h2>
            <p className="text-sm leading-relaxed text-neutral-600">{rival.pickMemdoor}</p>
          </div>
        </div>

        <h2 className="mb-6 text-center text-2xl font-bold tracking-tight text-neutral-900">Only in Memdoor</h2>
        <div className="mx-auto mb-14 grid max-w-5xl grid-cols-1 gap-4 sm:grid-cols-2">
          {[...WORKFLOWS, ...ONLY].map((r) => (
            <div key={r.feature} className="rounded-2xl border border-neutral-200 p-5">
              <div className="mb-1 font-semibold text-neutral-900">{r.feature}</div>
              <p className="text-sm leading-relaxed text-neutral-500">{r.detail}</p>
            </div>
          ))}
        </div>

        <h2 className="mb-3 text-center text-2xl font-bold tracking-tight text-neutral-900">Feature by feature</h2>
        <p className="mx-auto mb-8 max-w-2xl text-center text-sm text-neutral-500">
          A tick for {rival.name} means its own docs said so when we read them (28 September 2026); an empty cell
          means they did not say, not that it lacks the feature.
        </p>
        <div className="mx-auto max-w-4xl overflow-x-auto">
          <table className="w-full min-w-[32rem] border-collapse text-left text-sm">
            <thead>
              <tr className="border-b border-neutral-200 text-xs uppercase tracking-wider text-neutral-500">
                <th className="py-3 pr-4 font-medium">Feature</th>
                <th className="py-3 pr-4 font-medium">Memdoor</th>
                <th className="py-3 font-medium">{rival.name}</th>
              </tr>
            </thead>
            <tbody>
              {[...both, ...onlyMemdoor].map((r) => (
                <tr key={r.feature} className="border-b border-neutral-100 align-top">
                  <td className="py-3 pr-4 font-medium text-neutral-900">
                    {r.feature}
                    <div className="text-xs font-normal text-neutral-500">{r.detail}</div>
                  </td>
                  <td className="py-3 pr-4 text-neutral-900">{r.detail.startsWith('no,') ? '—' : '✓'}</td>
                  <td className="py-3 text-neutral-900">{has(r.also, rival.name) ? '✓' : ''}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="mx-auto mt-14 max-w-2xl text-center">
          <p className="mb-4 text-sm text-neutral-500">Free on your own key, any provider. One binary.</p>
          <InstallLine />
          <p className="mt-6 text-sm text-neutral-500">
            Other comparisons:{' '}
            {RIVALS.filter((r) => r.slug !== rival.slug).map((r, i, arr) => (
              <span key={r.slug}>
                <a className="text-neutral-900 underline underline-offset-4" href={`/compare/${r.slug}`}>{r.name}</a>
                {i < arr.length - 1 ? ' · ' : ''}
              </span>
            ))}
            {' · '}
            <a className="text-neutral-900 underline underline-offset-4" href="/features">all features</a>
          </p>
        </div>
      </main>
    </div>
  );
}
