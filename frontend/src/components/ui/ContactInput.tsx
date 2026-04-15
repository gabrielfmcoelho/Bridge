"use client";

import { useState, useRef, useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { contactsAPI } from "@/lib/api";

interface ContactInputProps {
  label?: string;
  value: string;
  onChange: (value: string) => void;
  type?: "name" | "phone";
}

// Strips everything except digits from a phone string.
function toRawDigits(val: string): string {
  return val.replace(/\D/g, "");
}

// Formats raw digits into (XX) XX 9 XXXX-XXXX mask.
// Handles partial input gracefully.
function formatPhone(raw: string): string {
  const d = raw.replace(/\D/g, "");
  if (d.length === 0) return "";
  if (d.length <= 2) return `(${d}`;
  if (d.length <= 4) return `(${d.slice(0, 2)}) ${d.slice(2)}`;
  if (d.length <= 5) return `(${d.slice(0, 2)}) ${d.slice(2, 4)} ${d.slice(4)}`;
  if (d.length <= 9) return `(${d.slice(0, 2)}) ${d.slice(2, 4)} ${d.slice(4, 5)} ${d.slice(5)}`;
  return `(${d.slice(0, 2)}) ${d.slice(2, 4)} ${d.slice(4, 5)} ${d.slice(5, 9)}-${d.slice(9, 13)}`;
}

// Formats raw digits for display in suggestions.
function displayPhone(raw: string): string {
  return formatPhone(raw) || raw;
}

export default function ContactInput({ label, value, onChange, type = "name" }: ContactInputProps) {
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [error, setError] = useState("");
  const ref = useRef<HTMLDivElement>(null);

  const { data: rawContacts } = useQuery({
    queryKey: ["contacts"],
    queryFn: contactsAPI.list,
  });
  const contacts = Array.isArray(rawContacts) ? rawContacts : [];

  const suggestions = type === "name"
    ? contacts.filter((c) => c.name && value && c.name.toLowerCase().includes(value.toLowerCase()) && c.name !== value).slice(0, 5)
    : contacts.filter((c) => c.phone && value && c.phone.includes(value) && c.phone !== value).slice(0, 5);

  const validatePhone = (raw: string) => {
    if (!raw) { setError(""); return; }
    if (raw.length > 0 && raw.length < 10) {
      setError("Phone too short");
    } else if (raw.length > 13) {
      setError("Phone too long");
    } else {
      setError("");
    }
  };

  useEffect(() => {
    const handleClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setShowSuggestions(false);
      }
    };
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  const handlePhoneChange = (inputValue: string) => {
    const raw = toRawDigits(inputValue).slice(0, 13); // max 13 digits
    onChange(raw);
    validatePhone(raw);
    setShowSuggestions(true);
  };

  // For phone type: value is raw digits, display is formatted
  const displayValue = type === "phone" ? formatPhone(value) : value;

  return (
    <div className="space-y-1.5 relative" ref={ref}>
      {label && (
        <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">
          {label}
        </label>
      )}
      <input
        type={type === "phone" ? "tel" : "text"}
        value={displayValue}
        onChange={(e) => {
          if (type === "phone") {
            handlePhoneChange(e.target.value);
          } else {
            onChange(e.target.value);
            setShowSuggestions(true);
          }
        }}
        onFocus={() => setShowSuggestions(true)}
        placeholder={type === "phone" ? "(XX) XX 9 XXXX-XXXX" : undefined}
        className={`w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border ${error ? "border-red-500" : "border-[var(--border-default)]"} rounded-[var(--radius-md)] px-3 py-2 text-sm transition-all duration-200 focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)] focus:outline-none`}
      />
      {error && <p className="text-xs text-red-400">{error}</p>}
      {showSuggestions && suggestions.length > 0 && (
        <div className="absolute z-20 left-0 right-0 mt-1 bg-[var(--bg-surface)] border border-[var(--border-default)] rounded-[var(--radius-md)] shadow-lg max-h-32 overflow-y-auto">
          {suggestions.map((c) => (
            <button
              key={c.id}
              type="button"
              onClick={() => {
                onChange(type === "name" ? c.name : c.phone);
                setShowSuggestions(false);
                setError("");
              }}
              className="w-full text-left px-3 py-1.5 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] transition-colors"
            >
              {type === "name" ? c.name : displayPhone(c.phone)}
              {type === "name" && c.phone && (
                <span className="text-[var(--text-faint)] ml-2 text-xs">{displayPhone(c.phone)}</span>
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
