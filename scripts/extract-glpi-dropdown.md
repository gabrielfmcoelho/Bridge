# Extracting GLPI dropdown options for sshcm's catalogue

The Formcreator drawer in sshcm needs to display picker options (category,
entity, location, etc.) from the GLPI instance. When the GLPI profile used by
sshcm has no REST read rights on those itemtypes, the picker comes back empty
or 403. Instead of fighting the permission model, admins **paste the option
list once** into sshcm's GLPI settings — the GLPI web UI renders the same
data, so we just read it from the rendered page.

Destination in sshcm: **Settings → Integrations → GLPI → Catálogo de dropdowns → Editar <itemtype>**.

## One-shot console snippet

1. In GLPI (web UI, logged in as a user who can *see* the dropdown you want),
   open any page that renders an `<option>` list for that itemtype:

   | Itemtype | Where in GLPI |
   |---|---|
   | `ITILCategory` | Configuration → Dropdowns → ITIL Categories (tree view). Or open any Formcreator form that uses the category picker and expand the dropdown. |
   | `Entity` | Administration → Entities (tree view). |
   | `Location` | Configuration → Dropdowns → Locations. |
   | `Supplier` | Management → Suppliers. |
   | `User` | Administration → Users. |
   | `Group` | Administration → Groups. |
   | `Software` | Assets → Software. |

2. Open DevTools (F12) → Console tab.
3. Paste and run:

   ```js
   copy(JSON.stringify(
     [...document.querySelectorAll('select option, .select2-results__option')]
       .map(o => ({
         id: parseInt(o.value || o.dataset.id || '', 10),
         name: (o.textContent || '').trim(),
       }))
       .filter(o => Number.isFinite(o.id) && o.id > 0 && o.name),
     null, 2
   ));
   ```

   `copy(...)` drops the JSON into your clipboard.

4. In sshcm, open the catalogue editor for that itemtype and paste into the
   textarea. Save.

## Hierarchical itemtypes (ITILCategory, Location)

The default snippet captures each option as `{id, name}`. For category/
location trees, you'll also want the full path ("ServiçosTI > Rede > DNS >
Criar"). GLPI's tree-view dropdown renders the full path as the option text —
the snippet above already picks that up. If you need to separate the leaf
from the path, use this variant instead:

```js
copy(JSON.stringify(
  [...document.querySelectorAll('select option, .select2-results__option')]
    .map(o => {
      const id = parseInt(o.value || o.dataset.id || '', 10);
      const full = (o.textContent || '').trim();
      // Last segment after " > " is the leaf; whole thing is the completename.
      const leaf = full.includes(' > ') ? full.split(' > ').pop() : full;
      return { id, name: leaf, completename: full };
    })
    .filter(o => Number.isFinite(o.id) && o.id > 0 && o.name),
  null, 2
));
```

## CSV fallback

GLPI's Administration interfaces have a CSV export. Download it, open in any
spreadsheet tool, keep only the `id` and `name` columns (plus `completename`
for trees), and convert to JSON before pasting. Any online CSV-to-JSON tool
works; `jq` can also do it with `jq -Rn '[inputs | split(",") | ...]'`.

## When to re-import

Option lists rarely change. Re-run the extraction only when the team creates
a new entity/category/user in GLPI. The catalogue page shows
`atualizado <timestamp>` for each itemtype — older than a few months is a
reasonable cue to refresh.

## Submissions still validate against GLPI

The catalogue is a **display** source. When the user picks an option, sshcm
submits that option's numeric id back to Formcreator. If the id has since
been deleted in GLPI, the submission will fail with `ERROR_GLPI_ADD` — fix
by re-importing the catalogue.
