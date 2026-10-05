// REMOTE CONTROL's transcript: how the page at /r/<id> folds the frames the
// terminal sends (cmd/cli/cmd/tui_remote.go) into lines. Kept apart from the
// page so the Go tests can run it against the frames the terminal really sends.

// An agent event as the gateway broadcasts it (the /ws agent_event data).
export type AgentEvent = {
  run_id?: string;
  stream?: string;
  data?: Record<string, unknown>;
};

export type Turn = {
  text?: string;
  question_id?: string;
  answer?: string;
  cancel?: boolean;
  history?: boolean;
  // The terminal view: its size (fresh starts the program over), and keys.
  tty?: { cols: number; rows: number; fresh?: boolean };
  tty_in?: string;
  // Ask for the screen so far, addressed to this page's random id.
  watch?: string;
};

export type Item =
  | { kind: "you"; text: string }
  | { kind: "reply"; run: string; text: string }
  | { kind: "tool"; text: string }
  | { kind: "note"; text: string }
  | {
      kind: "question";
      id: string;
      text: string;
      options: string[];
      answered?: string;
    };

// apply folds one event into the transcript: streamed text grows the run's
// reply instead of adding a line per token.
export function apply(items: Item[], ev: AgentEvent): Item[] {
  const d = ev.data ?? {};
  const event = String(d.event ?? "");
  const run = ev.run_id ?? "";
  const last = items[items.length - 1];
  switch (ev.stream) {
    case "assistant": {
      if (event === "text_delta" && typeof d.delta === "string") {
        if (last?.kind === "reply" && last.run === run) {
          return [
            ...items.slice(0, -1),
            { ...last, text: last.text + d.delta },
          ];
        }
        return [...items, { kind: "reply", run, text: d.delta }];
      }
      if (
        (event === "text_end" || event === "text") &&
        typeof d.text === "string" &&
        d.text
      ) {
        if (last?.kind === "reply" && last.run === run) {
          return [...items.slice(0, -1), { ...last, text: d.text }];
        }
        return [...items, { kind: "reply", run, text: d.text }];
      }
      return items;
    }
    case "tool":
      if (event === "start" && typeof d.tool === "string")
        return [...items, { kind: "tool", text: d.tool }];
      return items;
    case "question": {
      const options = Array.isArray(d.options)
        ? d.options.filter((o): o is string => typeof o === "string")
        : [];
      if (
        typeof d.question_id === "string" &&
        typeof d.question === "string" &&
        options.length
      ) {
        return [
          ...items,
          { kind: "question", id: d.question_id, text: d.question, options },
        ];
      }
      return items;
    }
    case "history": {
      // The conversation so far, sent when the page opens: it REPLACES what
      // is shown, so a reconnect never shows a turn twice.
      const messages = Array.isArray(d.messages) ? d.messages : [];
      const out: Item[] = [];
      for (const m of messages as { role?: unknown; text?: unknown }[]) {
        if (typeof m?.text !== "string" || !m.text) continue;
        out.push(
          m.role === "user"
            ? { kind: "you", text: m.text }
            : { kind: "reply", run: "", text: m.text },
        );
      }
      return out;
    }
    case "lifecycle":
      if (event === "error")
        return [
          ...items,
          { kind: "note", text: `error: ${String(d.error ?? "")}` },
        ];
      return items;
    default:
      return items;
  }
}
