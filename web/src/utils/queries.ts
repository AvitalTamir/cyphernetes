/**
 * Splits raw editor text into individual Cyphernetes queries, mirroring how the
 * backend lexer treats comments and string literals.
 *
 * The editor lets you run several `;`-separated queries at once. To split them
 * correctly we walk the text with a small state machine rather than regexes,
 * because comment and statement-separator characters must be ignored inside
 * string literals (e.g. `"http://x"`, `"a;b"`), and comments must act as token
 * separators (so `MAT/*x*\/CH` becomes `MAT CH`, two tokens, not `MATCH`).
 *
 * Comments are removed (replaced with a single space) so a flattened single
 * line can be sent to the backend; the backend lexer strips comments too, so
 * this only needs to keep the client-side split honest. An unterminated `/*` is
 * left verbatim and ends splitting, so the backend surfaces the syntax error.
 */
export function splitQueries(text: string): string[] {
  const queries: string[] = [];
  let current = '';
  let i = 0;
  const n = text.length;

  while (i < n) {
    const c = text[i];

    // String literal: copy verbatim, ignoring comment/`;` markers inside it.
    if (c === '"' || c === "'") {
      const quote = c;
      current += c;
      i++;
      while (i < n) {
        const ch = text[i];
        if (ch === '\\' && i + 1 < n) {
          current += ch + text[i + 1];
          i += 2;
          continue;
        }
        current += ch;
        i++;
        if (ch === quote) break;
      }
      continue;
    }

    // Single-line comment: drop through end of line, leaving a separator.
    if (c === '/' && text[i + 1] === '/') {
      i += 2;
      while (i < n && text[i] !== '\n') i++;
      current += ' ';
      continue;
    }

    // Multi-line comment: drop through the closing `*/`, leaving a separator.
    if (c === '/' && text[i + 1] === '*') {
      const close = text.indexOf('*/', i + 2);
      if (close === -1) {
        // Unterminated: keep the rest verbatim and stop splitting so the
        // backend reports the error (a `;` inside it must not split).
        current += text.slice(i);
        i = n;
      } else {
        i = close + 2;
        current += ' ';
      }
      continue;
    }

    // Statement separator.
    if (c === ';') {
      queries.push(current);
      current = '';
      i++;
      continue;
    }

    // Flatten newlines to spaces; copy everything else.
    current += c === '\n' ? ' ' : c;
    i++;
  }
  queries.push(current);

  return queries.map((q) => q.trim()).filter((q) => q.length > 0);
}
