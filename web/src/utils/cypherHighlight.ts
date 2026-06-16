import baseCypher from 'react-syntax-highlighter/dist/esm/languages/prism/cypher';

// Language name we register the extended grammar under. It is deliberately NOT
// "cypher": react-syntax-highlighter auto-registers the stock cypher grammar
// into the shared refractor instance, and refractor.register() refuses to
// overwrite an already-registered language. Registering under our own name
// guarantees our grammar (with multi-line comment support) is the one used.
export const CYPHER_LANGUAGE = 'cyphernetes';

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

type PrismLike = { languages: Record<string, Record<string, unknown>> };

// refractor/Prism syntax registrar: populates the stock cypher grammar, then
// registers a copy under CYPHER_LANGUAGE whose comment rule also matches
// multi-line `/* */` comments. Imports only `baseCypher` (a subpath of the
// direct react-syntax-highlighter dependency), so it resolves under pnpm's
// strict node_modules layout as well as npm's.
const cyphernetes = (prism: PrismLike) => {
  baseCypher(prism); // populates prism.languages.cypher
  prism.languages[CYPHER_LANGUAGE] = {
    ...prism.languages.cypher,
    comment: commentPatterns,
  };
};
cyphernetes.displayName = CYPHER_LANGUAGE;
cyphernetes.aliases = [] as string[];

export default cyphernetes;
