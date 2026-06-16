// The stock Cypher grammar function ships no type declarations.
declare module 'react-syntax-highlighter/dist/esm/languages/prism/cypher' {
  const cypher: (prism: { languages: Record<string, Record<string, unknown>> }) => void;
  export default cypher;
}
