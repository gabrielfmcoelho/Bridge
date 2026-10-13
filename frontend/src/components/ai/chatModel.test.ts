import { test } from "node:test";
import assert from "node:assert/strict";
import { citedRefs, historyFor, type ChatMsg } from "./chatModel.ts";

const host = { type: "host", id: "srv-sei-01", label: "srv-sei-01", href: "/hosts/srv-sei-01" };
const dns = { type: "dns", id: "4", label: "sei.pi.gov.br", href: "/dns/4" };
const unused = { type: "project", id: "9", label: "Portal", href: "/projects/9" };

test("cited refs: only the ones the answer links to, once, in answer order", () => {
  const answer = "O [sei.pi.gov.br](/dns/4) aponta para [srv-sei-01](/hosts/srv-sei-01); veja [srv-sei-01](/hosts/srv-sei-01) e [docs](https://x.y).";
  assert.deepEqual(citedRefs(answer, [host, dns, unused]), [dns, host]);
  assert.deepEqual(citedRefs("sem links", [host]), []);
  assert.deepEqual(citedRefs("[x](/hosts/srv-sei-01)"), []);
});

test("history: errors dropped, draft outcomes noted on the proposing turn", () => {
  const base = { title: "Renovar certificado", description: "", kind: "issue" as const };
  const msgs: ChatMsg[] = [
    { id: "1", role: "user", content: "crie uma issue" },
    { id: "2", role: "assistant", content: "Rascunho pronto.", proposals: [
      { ...base, id: "p1", status: "done", result: { label: "Issue #42", href: "/issues" } },
      { ...base, id: "p2", title: "Outra", status: "dismissed" },
      { ...base, id: "p3", title: "Pendente", status: "pending" },
    ] },
    { id: "3", role: "assistant", content: "timeout", error: true },
  ];
  assert.deepEqual(historyFor(msgs), [
    { role: "user", content: "crie uma issue" },
    { role: "assistant", content: 'Rascunho pronto.\n\n[O usuário confirmou o rascunho "Renovar certificado": Issue #42]\n\n[O usuário descartou o rascunho "Outra"]' },
  ]);
});
