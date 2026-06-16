import { describe, test, expect } from 'vitest';
import cypherWithBlockComments, { commentPatterns } from '../utils/cypherHighlight';

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

describe('cypherWithBlockComments registrar', () => {
  test('registers a cypher grammar whose comment rule covers block comments', () => {
    const prism: any = { languages: {} };
    cypherWithBlockComments(prism);
    expect(prism.languages.cypher).toBeDefined();
    const comment = prism.languages.cypher.comment;
    expect(Array.isArray(comment)).toBe(true);
    expect('/* x */'.match(comment[0].pattern)?.[0]).toBe('/* x */');
  });

  test('preserves the base cypher tokens (keyword, string)', () => {
    const prism: any = { languages: {} };
    cypherWithBlockComments(prism);
    expect(prism.languages.cypher.keyword).toBeDefined();
    expect(prism.languages.cypher.string).toBeDefined();
  });
});
