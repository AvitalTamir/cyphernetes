import { describe, test, expect } from 'vitest';
import refractor from 'refractor/core';
import { registerCypher, commentPatterns } from '../utils/cypherHighlight';

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

describe('registerCypher', () => {
  test('patches the live grammar so it has an array comment rule', () => {
    registerCypher();
    const comment = refractor.languages.cypher.comment;
    expect(Array.isArray(comment)).toBe(true);
  });

  test('refractor tokenizes a multi-line comment as a comment', () => {
    registerCypher();
    const tokens = JSON.stringify(refractor.highlight('/*\n a\n*/ MATCH (n)', 'cypher'));
    expect(tokens).toContain('"comment"');
  });

  test('refractor still tokenizes single-line comments and keywords', () => {
    registerCypher();
    const tokens = JSON.stringify(refractor.highlight('MATCH (n) // tail', 'cypher'));
    expect(tokens).toContain('"comment"');
    expect(tokens).toContain('"keyword"');
  });
});
