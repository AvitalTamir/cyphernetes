import { describe, test, expect } from 'vitest';
import cyphernetes, { commentPatterns, CYPHER_LANGUAGE } from '../utils/cypherHighlight';

const blockPattern = commentPatterns[0].pattern;
const linePattern = commentPatterns[1].pattern;

describe('cypher comment patterns', () => {
  test('block pattern matches an inline /* */ comment', () => {
    expect('/* hi */'.match(blockPattern)?.[0]).toBe('/* hi */');
  });

  test('block pattern matches a multi-line comment', () => {
    const c = '/*\n a\n b\n*/';
    expect(c.match(blockPattern)?.[0]).toBe(c);
  });

  test('block pattern is non-greedy across adjacent comments', () => {
    expect('/* a */ x /* b */'.match(blockPattern)?.[0]).toBe('/* a */');
  });

  test('block pattern matches an unterminated comment up to end of input', () => {
    expect('/* still typing'.match(blockPattern)?.[0]).toBe('/* still typing');
  });

  test('block pattern does not match plain code', () => {
    expect('MATCH (n) RETURN n'.match(blockPattern)).toBeNull();
  });

  test('line pattern still matches // comments', () => {
    expect('// hi'.match(linePattern)?.[0]).toBe('// hi');
  });
});

describe('cyphernetes grammar registrar', () => {
  // A minimal stand-in for the refractor/Prism instance.
  function fakePrism() {
    return { languages: {} as Record<string, Record<string, unknown>> };
  }

  test('registers under the CYPHER_LANGUAGE name with an array comment rule', () => {
    const prism = fakePrism();
    cyphernetes(prism);
    const grammar = prism.languages[CYPHER_LANGUAGE];
    expect(grammar).toBeDefined();
    expect(grammar.comment).toBe(commentPatterns);
  });

  test('preserves the base cypher tokens (keyword, string)', () => {
    const prism = fakePrism();
    cyphernetes(prism);
    const grammar = prism.languages[CYPHER_LANGUAGE];
    expect(grammar.keyword).toBeDefined();
    expect(grammar.string).toBeDefined();
  });

  test('does not clobber the stock cypher grammar', () => {
    const prism = fakePrism();
    cyphernetes(prism);
    expect(prism.languages.cypher).toBeDefined();
    expect(prism.languages.cypher.comment).not.toBe(commentPatterns);
  });
});
