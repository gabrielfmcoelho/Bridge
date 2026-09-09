"use client";

import { useState } from "react";
import { Section, Specimen } from "./Section";
import Input from "@/components/ui/Input";
import Textarea from "@/components/ui/Textarea";
import Select from "@/components/ui/Select";
import Checkbox from "@/components/ui/Checkbox";
import CheckboxList from "@/components/ui/CheckboxList";
import RadioGroup from "@/components/ui/RadioGroup";
import Toggle from "@/components/ui/Toggle";
import DateTimeInput from "@/components/ui/DateTimeInput";
import TagInput from "@/components/ui/TagInput";
import AsyncPicker, { type AsyncPickerItem } from "@/components/ui/AsyncPicker";
import ContactInput from "@/components/ui/ContactInput";
import MarkdownEditor from "@/components/ui/MarkdownEditor";
import FormError from "@/components/ui/FormError";
import StepIndicator from "@/components/ui/StepIndicator";
import EntidadeScopeFields, { defaultGrants } from "@/components/entidades/EntidadeScopeFields";
import PasswordField from "@/app/hosts/[slug]/_components/PasswordField";
import { useAuth } from "@/contexts/AuthContext";
import type { AssetGrantsInput } from "@/lib/types";

const noop = () => {};

// AsyncPicker keeps `fetcher` in a useEffect dep array — an inline lambda would
// re-run the debounced search on every render while the popover is open.
const PICKER_ITEMS: AsyncPickerItem[] = [
  { id: 1, label: "web-01", secondary: "10.0.0.11" },
  { id: 2, label: "db-01", secondary: "10.0.0.12" },
  { id: 3, label: "cache-01", secondary: "10.0.0.13" },
];
const pickerFetcher = async (q: string) =>
  PICKER_ITEMS.filter((i) => i.label.toLowerCase().includes(q.toLowerCase()));

const OPTIONS_3 = [
  { value: "prod", label: "Production" },
  { value: "stg", label: "Staging" },
  { value: "dev", label: "Development" },
];
const OPTIONS_7 = [
  { value: "a", label: "AlmaLinux" },
  { value: "b", label: "Debian" },
  { value: "c", label: "Rocky Linux" },
  { value: "d", label: "Ubuntu" },
  { value: "e", label: "Windows Server" },
  { value: "f", label: "FreeBSD" },
  { value: "g", label: "Alpine" },
];
const ITEMS_4 = [
  { id: 1, name: "SEAD" },
  { id: 2, name: "SEFAZ" },
  { id: 3, name: "SEDUC" },
  { id: 4, name: "SESAPI" },
];
const ITEMS_10 = Array.from({ length: 10 }, (_, i) => ({ id: i + 1, name: `Entidade ${i + 1}` }));

// Verified counts of every distinct <label className=…> string in src/.
const LABEL_CLASSES: [string, number][] = [
  ["block text-xs font-medium text-[var(--text-secondary)] tracking-wide", 17],
  ["block text-xs text-[var(--text-muted)] mb-1", 10],
  ["text-xs font-medium text-[var(--text-muted)] block", 6],
  ["block text-sm mb-1.5 text-[var(--text-secondary)]", 6],
  ["text-xs font-medium text-[var(--text-muted)] mb-1.5 block", 5],
  ["flex items-center gap-2 text-xs text-[var(--text-secondary)]", 4],
  ["block text-xs font-medium text-[var(--text-secondary)] tracking-wide mb-1.5", 4],
  ["block text-xs font-medium tracking-wide text-[var(--text-secondary)]", 3],
  ["block text-xs font-medium text-[var(--text-secondary)] mb-1", 3],
];

export default function FormsSection() {
  const { user } = useAuth();

  const [select3, setSelect3] = useState("prod");
  const [select7, setSelect7] = useState("");
  const [checked, setChecked] = useState(true);
  const [unchecked, setUnchecked] = useState(false);
  const [list4, setList4] = useState<number[]>([2]);
  const [list10, setList10] = useState<number[]>([1, 3]);
  const [radio, setRadio] = useState("weekly");
  const [toggleOn, setToggleOn] = useState(true);
  const [toggleOff, setToggleOff] = useState(false);
  const [date, setDate] = useState("2026-09-09");
  const [datetime, setDatetime] = useState("2026-09-09 14:30:00");
  const [time, setTime] = useState("14:30:00");
  const [tags, setTags] = useState<string[]>(["prod"]);
  const [picked, setPicked] = useState<AsyncPickerItem | null>(null);
  const [pickedMany, setPickedMany] = useState<AsyncPickerItem[]>([]);
  const [contactName, setContactName] = useState("");
  const [contactPhone, setContactPhone] = useState("");
  const [markdown, setMarkdown] = useState("# Runbook\n\nRestart with `systemctl restart nginx`.");
  const [grants, setGrants] = useState<AssetGrantsInput>(() => defaultGrants(user));
  const [grantsCompact, setGrantsCompact] = useState<AssetGrantsInput>(() => defaultGrants(user));

  return (
    <Section id="forms" title="Inputs & Forms">
      <Specimen title="Input" source="components/ui/Input.tsx" wide>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <Input label="Hostname" placeholder="web-01" />
          <Input label="Hostname" placeholder="web-01" error="Required" />
          <Input label="Hostname" placeholder="web-01" disabled />
        </div>
      </Specimen>

      <Specimen title="Textarea" source="components/ui/Textarea.tsx" wide>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Textarea label="Notes" rows={3} placeholder="Free text…" />
          <Textarea label="Notes" rows={3} error="Too long" />
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Error border and text read <code>--danger</code>, same as <code>Input</code>.
        </p>
      </Specimen>

      <Specimen title="Select" source="components/ui/Select.tsx" wide>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Select
            label="Environment (3 options)"
            options={OPTIONS_3}
            value={select3}
            onChange={(e) => setSelect3(e.target.value)}
          />
          <Select
            label="Distro (7 options — searchable)"
            options={OPTIONS_7}
            value={select7}
            onChange={(e) => setSelect7(e.target.value)}
            searchPlaceholder="Search distro..."
          />
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Radix popover, not a native <code>&lt;select&gt;</code>. Over 5 options adds a search box.{" "}
          <code>onChange</code> receives a fake <code>{"{ target: { value } }"}</code> so call sites read like a
          native input.
        </p>
      </Specimen>

      <Specimen title="Checkbox" source="components/ui/Checkbox.tsx">
        <Checkbox label="Monitored" checked={checked} onChange={setChecked} />
        <Checkbox label="Deprecated" checked={unchecked} onChange={setUnchecked} />
        <Checkbox label="Locked" checked disabled onChange={noop} />
      </Specimen>
      <p className="text-xs text-[var(--text-muted)]">
        The box visuals come from the global <code>input[type=checkbox]</code> rule in{" "}
        <code>app/globals.css</code>, not from this component.
      </p>

      <Specimen title="CheckboxList" source="components/ui/CheckboxList.tsx" wide>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <CheckboxList label="Responsibles (4)" items={ITEMS_4} selected={list4} onChange={setList4} />
          <CheckboxList label="Responsibles (10)" items={ITEMS_10} selected={list10} onChange={setList10} />
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Over 8 items adds a search box; a non-empty selection adds the{" "}
          <code>N selected</code> counter next to the label.
        </p>
      </Specimen>

      <Specimen title="RadioGroup" source="components/ui/RadioGroup.tsx">
        <RadioGroup
          label="Scan cadence"
          name="ds-cadence"
          value={radio}
          onChange={setRadio}
          options={[
            { value: "daily", label: "Daily" },
            { value: "weekly", label: "Weekly" },
            { value: "monthly", label: "Monthly" },
          ]}
        />
        <RadioGroup
          label="Scan cadence"
          name="ds-cadence-err"
          value=""
          onChange={noop}
          error="Pick one"
          options={[
            { value: "daily", label: "Daily" },
            { value: "weekly", label: "Weekly" },
            { value: "monthly", label: "Monthly" },
          ]}
        />
      </Specimen>

      <Specimen title="Toggle" source="components/ui/Toggle.tsx">
        <Toggle checked={toggleOn} onChange={setToggleOn} ariaLabel="On" />
        <Toggle checked={toggleOff} onChange={setToggleOff} ariaLabel="Off" />
        <Toggle checked disabled onChange={noop} ariaLabel="Disabled" />
      </Specimen>

      <Specimen title="DateTimeInput" source="components/ui/DateTimeInput.tsx" wide>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <DateTimeInput variant="date" label="Date" value={date} onChange={setDate} />
          <DateTimeInput variant="datetime" label="Datetime" value={datetime} onChange={setDatetime} />
          <DateTimeInput variant="time" label="Time" value={time} onChange={setTime} />
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Thin wrapper over the native inputs that normalises to and from the GLPI wire format —{" "}
          <code>YYYY-MM-DD HH:MM:SS</code> for datetime, <code>HH:MM:SS</code> for time.
        </p>
      </Specimen>

      <Specimen title="TagInput" source="components/ui/TagInput.tsx" wide>
        <TagInput label="Tags" tags={tags} onChange={setTags} suggestions={["prod", "staging", "legacy"]} />
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Passing <code>suggestions</code> sets <code>enabled: !externalSuggestions</code> on its{" "}
          <code>tagsAPI.list</code> query, so this instance issues no request.
        </p>
      </Specimen>

      <Specimen title="AsyncPicker" source="components/ui/AsyncPicker.tsx" wide>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <AsyncPicker
            label="Host (single)"
            fetcher={pickerFetcher}
            value={picked?.id ?? null}
            selectedLabel={picked?.label}
            onChange={setPicked}
          />
          <AsyncPicker
            label="Hosts (multi)"
            fetcher={pickerFetcher}
            multi
            selectedItems={pickedMany}
            onChange={setPickedMany}
          />
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Discriminated union on <code>multi</code>: single mode stores the id and needs{" "}
          <code>selectedLabel</code> to redraw; multi mode makes the caller own the full items so labels and
          colours survive without a re-fetch. <code>fetcher</code> sits in a <code>useEffect</code> dep array —
          it must be a stable reference.
        </p>
      </Specimen>

      <Specimen title="ContactInput" source="components/ui/ContactInput.tsx" wide>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <ContactInput type="name" label="Responsible" value={contactName} onChange={setContactName} />
          <ContactInput type="phone" label="Phone" value={contactPhone} onChange={setContactPhone} />
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Both instances hit <code>contactsAPI.list</code>. Under <code>dev:mock</code> that 404s (three
          react-query retries in the console) and the field degrades to no suggestions — the mask and validation
          still work.
        </p>
      </Specimen>

      <Specimen title="MarkdownEditor" source="components/ui/MarkdownEditor.tsx" wide>
        <MarkdownEditor label="Description" value={markdown} onChange={setMarkdown} rows={4} />
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Write/Preview tabs over one textarea. <code>renderMarkdown()</code> in the same file is the single
          sanitized <code>marked</code> → <code>DOMPurify</code> pipeline used by the wiki and the public share
          page.
        </p>
      </Specimen>

      <Specimen title="EntidadeScopeFields" source="components/entidades/EntidadeScopeFields.tsx" wide>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div>
            <p className="text-[11px] text-[var(--text-faint)] mb-2">default</p>
            <EntidadeScopeFields value={grants} onChange={setGrants} />
          </div>
          <div>
            <p className="text-[11px] text-[var(--text-faint)] mb-2">compact</p>
            <EntidadeScopeFields value={grantsCompact} onChange={setGrantsCompact} compact />
          </div>
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Composes <code>Select</code> + <code>CheckboxList</code> + <code>Toggle</code> and fetches{" "}
          <code>entidadesAPI.list</code> (served by Go and by the mock). 12 forms embed it; the parent owns the{" "}
          <code>AssetGrantsInput</code> and spreads it into the payload. <code>defaultGrants(user)</code> seeds
          creator from the user&apos;s primary entidade.
        </p>
      </Specimen>

      <Specimen
        title="FormError"
        source="components/ui/FormError.tsx"
        alsoIn={["components/requests/DynamicField.tsx:64 (per-field, not form-level)"]}
        wide
      >
        <FormError message="Hostname already exists" />
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Form-level banner on <code>--danger</code>. There is no toast component — banners are <code>StatusAlert</code>,
          inline errors are this or an input&apos;s own <code>error</code> prop.
        </p>
      </Specimen>

      <Specimen title="StepIndicator" source="components/ui/StepIndicator.tsx" wide>
        <StepIndicator steps={["Basics", "Scope", "Review"]} current={2} />
        <p className="text-xs text-[var(--text-muted)] mt-3">
          <code>current</code> is 1-based and at most two steps are drawn at a time.{" "}
          <code>hooks/useMultiStepForm.tsx</code> → <code>useMultiStepFormEffects</code> pushes it into the
          modal <code>subHeader</code> slot and Back/Next/Save into <code>footer</code>.
        </p>
      </Specimen>

      <Specimen title="PasswordField" source="app/hosts/[slug]/_components/PasswordField.tsx" wide>
        {/* ponytail: demo slug — clicking the eye hits a 404 and exercises the error path */}
        <PasswordField slug="demo-host" />
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Three password-reveal implementations: this one, <code>hooks/useSecretReveal.ts</code> plus three
          widget copies, and a plain <code>&lt;Input type=&quot;password&quot;&gt;</code> at 13 sites.
        </p>
      </Specimen>

      <Specimen title="<label> class permutations" source={"grep <label className> across src"} wide>
        <div className="space-y-3">
          {LABEL_CLASSES.map(([cls, count], i) => (
            <div key={cls} className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
              <span className={cls}>Label text</span>
              <code className="text-[11px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
                {cls}
              </code>
              <span className="text-[11px] text-[var(--text-muted)] tabular-nums">{count}&times;</span>
              {i === 0 && (
                <span className="text-[10px] px-1.5 py-0.5 rounded-[var(--radius-sm)] bg-[var(--accent-muted)] text-[var(--accent)] font-semibold">
                  canonical
                </span>
              )}
            </div>
          ))}
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Nine distinct strings for one element. Row 1 is what every <code>components/ui</code> input emits;
          rows 3 and 8 differ from it only by class order or token.
        </p>
      </Specimen>

      <Specimen
        title="Required-field asterisk"
        source="components/requests/DynamicField.tsx:58"
        alsoIn={[
          "app/requests/[id]/_components/TransitionModal.tsx:105",
          "app/requests/[id]/_components/RequestEditModal.tsx:117",
          "app/catalog/_components/RequestFormModal.tsx:299",
          "app/catalog/_components/RequestFormModal.tsx:368",
          "app/secrets/_components/AppLoginForm.tsx:40 (still text-red-400)",
        ]}
      >
        {/* specimen: components/requests/DynamicField.tsx:58 */}
        <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">
          Name<span className="text-[var(--danger)] ml-0.5">*</span>
        </label>
        {/* specimen: app/secrets/_components/NewSecretModal.tsx:44 — copied, NOT imported (guardrailed path) */}
        <label className="text-xs font-medium text-[var(--text-muted)] block">
          Name<span className="text-red-400 ml-0.5">*</span>
        </label>
      </Specimen>
      <p className="text-xs text-[var(--text-muted)]">
        The two <code>text-red-400</code> sites are byte-identical private <code>FormRow</code> components (
        <code>NewSecretModal.tsx:29</code>, <code>AppLoginForm.tsx:25</code>) — the token version above is the
        one that follows the theme.
      </p>

      <Specimen title="Field wrapper + helper text" source="components/requests/DynamicField.tsx:53-63" wide>
        {/* specimen: components/requests/DynamicField.tsx:53-63 */}
        <div className="space-y-1.5">
          <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">
            Slug<span className="text-[var(--danger)] ml-0.5">*</span>
          </label>
          <Input placeholder="meu-servico" />
          <p className="text-xs text-[var(--text-muted)]">Helper text</p>
          <p className="text-xs text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
            ^[a-z0-9-]+$
          </p>
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-3">
          The <code>space-y-1.5</code> wrapper is repeated inline inside every input primitive;{" "}
          <code>DynamicField.wrap()</code> is the only place it was factored out.
        </p>
      </Specimen>

      <div className="space-y-2">
        <h3 className="text-sm font-semibold text-[var(--text-primary)]">Gaps (described, not rendered)</h3>
        <ul className="text-xs text-[var(--text-muted)] list-disc pl-5 space-y-1">
          <li>
            No shared file upload — 5 raw <code>type=&quot;file&quot;</code>:{" "}
            <code>components/atlas/apis/AddApiModal.tsx:139</code>,{" "}
            <code>components/glpi/FormcreatorFileInput.tsx:80</code>, <code>app/settings/page.tsx:862</code>,{" "}
            <code>:1068</code>, <code>:1301</code>.
          </li>
          <li>
            No shared search box beyond <code>ListToolbar</code>: <code>components/wiki/WikiSearchBar.tsx</code>,{" "}
            <code>components/lineage/SearchOmnibar.tsx</code>,{" "}
            <code>app/catalog/_components/CatalogSearch.tsx</code>, <code>components/lineage/TablesPanel.tsx</code>,
            plus the private ones inside <code>Select</code>, <code>CheckboxList</code> and{" "}
            <code>AsyncPicker</code>. The 300 ms debounce is duplicated in{" "}
            <code>CatalogSearch.tsx:59</code> and <code>app/requests/_components/RequestList.tsx:58</code> — no{" "}
            <code>useDebounce</code>.
          </li>
          <li>
            Code input is <code>&lt;Textarea className=&quot;font-mono&quot;&gt;</code>:{" "}
            <code>app/ssh-keys/page.tsx:270</code>,{" "}
            <code>components/atlas/apis/ProjectSecretsSheet.tsx:62</code>.
          </li>
          <li>
            Canonical form wiring: <code>app/dns/DnsForm.tsx</code> — <code>useState</code> per field →{" "}
            <code>useMutation</code> → <code>&lt;FormError&gt;</code> →{" "}
            <code>loading={"{mutation.isPending}"}</code>. <code>hasExternalFooter</code> makes Dns/Host/Service/
            ProjectForm render two action bars.
          </li>
          <li>
            Raw controls bypassing <code>ui/</code>: 55 <code>&lt;input&gt;</code> (13 of them inside{" "}
            <code>ui/</code>), 12 <code>&lt;textarea&gt;</code>, 22 <code>&lt;select&gt;</code>. Heaviest:{" "}
            <code>components/atlas/apis/ShareBundleModal.tsx</code> (6), <code>app/settings/page.tsx</code> (4),{" "}
            <code>app/hosts/[slug]/_components/SSHOperations.tsx</code> (4).
          </li>
          <li>
            Name collision: <code>components/ui/Field.tsx</code> is a read-only label/value display (shown in
            Tables &amp; Lists), not a form field; <code>app/ssh-keys/page.tsx:469</code> defines an unrelated
            local <code>Field</code>.
          </li>
        </ul>
      </div>
    </Section>
  );
}
