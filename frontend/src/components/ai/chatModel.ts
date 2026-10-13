// The assistant thread's data and the pure rules over it. Runtime-import
// free so `node --test chatModel.test.ts` runs it.
import type { AiProposal, AiRef, AiStep } from "@/lib/types";

/** A drafted action and what the user did with it. */
export interface ProposalState extends AiProposal {
  status: "pending" | "done" | "dismissed";
  /** After confirming: what was created, e.g. "Issue #42" → /issues. */
  result?: { label: string; href?: string };
}

export interface ChatMsg {
  id: string;
  role: "user" | "assistant";
  content: string;
  /** The request failed; `content` is the error detail. Never sent back as history. */
  error?: boolean;
  steps?: AiStep[];
  refs?: AiRef[];
  proposals?: ProposalState[];
  /** The model gateway refused tools, so this answer didn't look at the data. */
  noTools?: boolean;
}

/** The refs the answer actually links to ("](/hosts/x)"), once each, in answer order. */
export function citedRefs(content: string, refs: AiRef[] = []): AiRef[] {
  const out: AiRef[] = [];
  const seen = new Set<string>();
  for (const m of content.matchAll(/\]\(([^)\s]+)\)/g)) {
    const r = refs.find((x) => x.href === m[1]);
    if (r && !seen.has(r.href)) {
      seen.add(r.href);
      out.push(r);
    }
  }
  return out;
}

/** The conversation as the model gets it back: failed turns dropped, and what
 *  the user did with each draft noted on the turn that proposed it. */
export function historyFor(messages: ChatMsg[]): { role: "user" | "assistant"; content: string }[] {
  return messages
    .filter((m) => !m.error)
    .map((m) => {
      const notes = (m.proposals ?? [])
        .filter((p) => p.status !== "pending")
        .map((p) => (p.status === "done"
          ? `[O usuário confirmou o rascunho "${p.title}": ${p.result?.label ?? "criado"}]`
          : `[O usuário descartou o rascunho "${p.title}"]`));
      return { role: m.role, content: [m.content, ...notes].filter(Boolean).join("\n\n") };
    });
}
