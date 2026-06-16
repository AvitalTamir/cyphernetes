/**
 * Splits raw editor text into individual Cyphernetes queries.
 *
 * The editor text is flattened onto a single line (newlines become spaces)
 * and then split on ';' to support running multiple queries at once. That
 * flattening means comments have to be removed here first:
 *   - A single-line '//' comment would otherwise swallow the rest of the
 *     flattened query.
 *   - A ';' (or '//') inside a comment would otherwise corrupt the split.
 * Multi-line slash-star comments are stripped before single-line ones so that
 * any ';' or '//' they contain cannot affect the later steps.
 *
 * The backend lexer also strips comments, so this is only about keeping the
 * client-side flatten/split honest — it is not the source of truth.
 */
export function splitQueries(text: string): string[] {
  const normalized = text
    // Remove multi-line comments first (non-greedy, so they do not nest and
    // adjacent comments are removed independently).
    .replaceAll(/\/\*[\s\S]*?\*\//g, '')
    // Remove single-line comments (up to the end of the line).
    .replaceAll(/\/\/.*\n/g, '')
    // Flatten remaining newlines into spaces.
    .replace(/\n/g, ' ');

  return normalized
    .replace(/;$/, '')
    .split(';')
    .map((q) => q.trim());
}
