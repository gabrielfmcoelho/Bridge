"use client";

import { useEffect, useState } from "react";

// Trailing debounce of a value: the caller keeps echoing `value` instantly and
// reads the returned one for anything expensive (queries, URL sync).
export function useDebounce<T>(value: T, ms = 300): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), ms);
    return () => clearTimeout(id);
  }, [value, ms]);
  return debounced;
}
