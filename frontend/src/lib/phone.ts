/**
 * Brazilian phone numbers, stored as digits: "86999998888" (DDD + number),
 * optionally with the country code ("5586999998888"). Formatting follows the
 * digits as they're typed, so it also works as an input mask.
 */
export const phoneDigits = (s: string) => s.replace(/\D/g, "").slice(0, 13);

export function formatPhone(raw: string): string {
  let d = phoneDigits(raw);
  if (!d) return "";
  let cc = "";
  if (d.length > 11) {
    cc = `+${d.slice(0, d.length - 11)} `;
    d = d.slice(-11);
  }
  if (d.length <= 2) return `${cc}(${d}`;
  const ddd = `(${d.slice(0, 2)}) `;
  const n = d.slice(2);
  if (n.length <= 4) return cc + ddd + n;
  if (n.length <= 8) return cc + ddd + `${n.slice(0, 4)}-${n.slice(4)}`; // landline
  return cc + ddd + `${n.slice(0, 1)} ${n.slice(1, 5)}-${n.slice(5)}`; // mobile
}

/** "" when fine (or empty); otherwise the i18n key of the problem. */
export function phoneProblem(raw: string): string {
  const n = phoneDigits(raw).length;
  if (n === 0) return "";
  if (n < 10) return "contact.phoneTooShort";
  return "";
}

/** wa.me wants the country code; assume Brazil when it's missing. */
export const whatsappNumber = (raw: string) => {
  const d = phoneDigits(raw);
  return d.length <= 11 ? `55${d}` : d;
};
