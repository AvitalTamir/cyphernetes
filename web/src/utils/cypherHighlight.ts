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

// refractor/Prism syntax registrar. Registers the stock Cypher grammar (which
// only recognizes single-line `//` comments) and then swaps its comment rule
// for one that also covers multi-line block comments.
const cypherWithBlockComments = (prism: any) => {
  baseCypher(prism);
  if (prism.languages?.cypher) {
    prism.languages.cypher.comment = commentPatterns;
  }
};
cypherWithBlockComments.displayName = 'cypher';
cypherWithBlockComments.aliases = [] as string[];

export default cypherWithBlockComments;
