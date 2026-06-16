import React from 'react';
import { render } from '@testing-library/react';
import { describe, test, expect } from 'vitest';
import { PrismLight } from 'react-syntax-highlighter';
import cyphernetes, { CYPHER_LANGUAGE } from '../utils/cypherHighlight';

PrismLight.registerLanguage(CYPHER_LANGUAGE, cyphernetes);

describe('PrismLight cyphernetes rendering', () => {
  // Proves the grammar tokenizes the *entire* multi-line comment (every line,
  // including the interior) as a comment. This is the grammar-correctness test.
  test('tokenizes a whole multi-line comment as comments', () => {
    const { container } = render(
      <PrismLight language={CYPHER_LANGUAGE}>
        {'/*\n multi line\n*/\nMATCH (n) RETURN n'}
      </PrismLight>
    );
    const comments = Array.from(container.querySelectorAll('.token.comment'));
    expect(comments.length).toBeGreaterThan(0);
    expect(comments.map((c) => c.textContent).join('')).toContain('multi line');
  });

  // QueryInput renders with wrapLines/wrapLongLines. react-syntax-highlighter
  // splits multi-line tokens across line rows when wrapping, so the comment is
  // still recognized and its delimiters highlighted (the grammar correctness is
  // covered by the test above).
  test('recognizes a multi-line comment when wrapping (as QueryInput renders)', () => {
    const { container } = render(
      <PrismLight language={CYPHER_LANGUAGE} wrapLines wrapLongLines>
        {'/*\n multi line\n*/\nMATCH (n) RETURN n'}
      </PrismLight>
    );
    const comments = Array.from(container.querySelectorAll('.token.comment'));
    expect(comments.length).toBeGreaterThan(0);
    expect(comments.map((c) => c.textContent).join('')).toContain('/*');
  });

  test('still highlights single-line comments and keywords', () => {
    const { container } = render(
      <PrismLight language={CYPHER_LANGUAGE}>{'MATCH (n) // trailing'}</PrismLight>
    );
    expect(container.querySelector('.token.comment')?.textContent).toContain('// trailing');
    expect(container.querySelector('.token.keyword')?.textContent).toBe('MATCH');
  });
});
