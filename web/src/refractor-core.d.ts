// react-syntax-highlighter's PrismLight uses this shared refractor instance.
// refractor/core ships no type declarations, so declare the bits we use.
declare module 'refractor/core' {
  interface Refractor {
    register: (grammar: (prism: unknown) => void) => void;
    registered: (language: string) => boolean;
    highlight: (value: string, language: string) => unknown;
    languages: Record<string, Record<string, unknown>>;
  }
  const refractor: Refractor;
  export default refractor;
}

declare module 'react-syntax-highlighter/dist/esm/languages/prism/cypher' {
  const cypher: (prism: unknown) => void;
  export default cypher;
}
