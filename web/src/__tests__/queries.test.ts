import { describe, test, expect } from 'vitest';
import { splitQueries } from '../utils/queries';

const collapse = (s: string) => s.replace(/\s+/g, ' ').trim();

describe('splitQueries', () => {
  test('returns a single query unchanged', () => {
    expect(splitQueries('MATCH (n) RETURN n')).toEqual(['MATCH (n) RETURN n']);
  });

  test('splits multiple queries on semicolons', () => {
    expect(splitQueries('MATCH (a) RETURN a; MATCH (b) RETURN b')).toEqual([
      'MATCH (a) RETURN a',
      'MATCH (b) RETURN b',
    ]);
  });

  test('strips single-line comments', () => {
    const result = splitQueries('MATCH (n) // grab everything\nRETURN n');
    expect(result).toHaveLength(1);
    expect(collapse(result[0])).toBe('MATCH (n) RETURN n');
  });

  test('strips multi-line comments spanning lines', () => {
    const input = 'MATCH (d:Deployment)\n/*\nThis is a multi-line comment\nWoot\n*/\nRETURN d.spec.replicas';
    const result = splitQueries(input);
    expect(result).toHaveLength(1);
    expect(collapse(result[0])).toBe('MATCH (d:Deployment) RETURN d.spec.replicas');
  });

  test('strips inline multi-line comments', () => {
    const result = splitQueries('MATCH (p:Pod) /* inline */ RETURN p');
    expect(result).toHaveLength(1);
    expect(collapse(result[0])).toBe('MATCH (p:Pod) RETURN p');
  });

  test('a semicolon inside a multi-line comment does not split the query', () => {
    const result = splitQueries('MATCH (p:Pod) /* tip: use ; sparingly */ RETURN p');
    expect(result).toHaveLength(1);
    expect(collapse(result[0])).toBe('MATCH (p:Pod) RETURN p');
  });

  test('multiple multi-line comments are each removed (no nesting)', () => {
    const result = splitQueries('MATCH (p:Pod) /* a */ WHERE p.x = 1 /* b */ RETURN p');
    expect(result).toHaveLength(1);
    expect(collapse(result[0])).toBe('MATCH (p:Pod) WHERE p.x = 1 RETURN p');
  });

  test('comment markers inside a double-quoted string are not stripped', () => {
    const result = splitQueries('MATCH (n) WHERE n.x = "/* not a comment */ // nope" RETURN n');
    expect(result).toHaveLength(1);
    expect(collapse(result[0])).toBe('MATCH (n) WHERE n.x = "/* not a comment */ // nope" RETURN n');
  });

  test('comment markers inside a single-quoted string are not stripped', () => {
    const result = splitQueries("MATCH (n) WHERE n.x = '/* keep */' RETURN n");
    expect(result).toHaveLength(1);
    expect(collapse(result[0])).toBe("MATCH (n) WHERE n.x = '/* keep */' RETURN n");
  });

  test('a semicolon inside a string does not split the query', () => {
    const result = splitQueries('MATCH (n) WHERE n.x = "a;b" RETURN n');
    expect(result).toHaveLength(1);
    expect(collapse(result[0])).toBe('MATCH (n) WHERE n.x = "a;b" RETURN n');
  });

  test('a comment acts as a token separator (does not concatenate tokens)', () => {
    expect(splitQueries('MAT/*x*/CH')).toEqual(['MAT CH']);
  });

  test('a trailing semicolon followed by whitespace produces no empty query', () => {
    expect(splitQueries('MATCH (n) RETURN n; ')).toEqual(['MATCH (n) RETURN n']);
    expect(splitQueries('MATCH (n) RETURN n;\n')).toEqual(['MATCH (n) RETURN n']);
  });

  test('a comment-only input produces no queries', () => {
    expect(splitQueries('/* just a note */')).toEqual([]);
    expect(splitQueries('// just a note\n')).toEqual([]);
  });

  test('an unterminated multi-line comment is kept verbatim and does not split', () => {
    // Kept as a single query so the backend lexer surfaces the syntax error;
    // a ';' inside the unterminated comment must not split it.
    const result = splitQueries('MATCH (n) /* oops ; still open');
    expect(result).toHaveLength(1);
    expect(result[0]).toContain('/* oops ; still open');
  });
});
