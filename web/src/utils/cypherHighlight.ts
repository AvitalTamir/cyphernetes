import refractor from 'refractor/core';
import baseCypher from 'react-syntax-highlighter/dist/esm/languages/prism/cypher';

// Prism token patterns for Cypher comments.
//
// The block-comment rule is listed first so that at a slash-star position the
// multi-line rule wins; a `//` falls through to the single-line rule. The
// block pattern also matches an unterminated comment up to end-of-input (the
// `|$` alternative) so a half-typed comment still renders as a comment while
// the user is typing.
export const commentPatterns = [
  { pattern: /\/\*[\s\S]*?(?:\*\/|$)/, greedy: true },
  { pattern: /\/\/.*/, greedy: true },
];

// Make refractor's Cypher grammar recognize multi-line `/* */` comments.
//
// react-syntax-highlighter registers the stock Cypher grammar (whose comment
// rule only matches single-line `//`) into a shared refractor instance, and
// refractor.register() refuses to overwrite an already-registered language.
// So registering an "extended" grammar is a silent no-op. Instead we ensure the
// base grammar exists and then patch its comment rule directly on the live
// grammar object that the highlighter actually uses.
export function registerCypher(): void {
  refractor.register(baseCypher);
  const grammar = refractor.languages.cypher;
  if (grammar) {
    grammar.comment = commentPatterns;
  }
}
