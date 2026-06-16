import React from 'react';
import { render } from '@testing-library/react';
import { describe, test, expect } from 'vitest';
import { PrismLight } from 'react-syntax-highlighter';
import cyphernetes, { CYPHER_LANGUAGE } from '../utils/cypherHighlight';

PrismLight.registerLanguage(CYPHER_LANGUAGE, cyphernetes);

describe('PrismLight cyphernetes rendering', () => {
  test('renders a multi-line comment as comment tokens', () => {
    const { container } = render(
      <PrismLight language={CYPHER_LANGUAGE}>
        {'/*\n multi line\n*/\nMATCH (n) RETURN n'}
      </PrismLight>
    );
    const comments = Array.from(container.querySelectorAll('.token.comment'));
    expect(comments.length).toBeGreaterThan(0);
    expect(comments.map((c) => c.textContent).join('')).toContain('multi line');
  });

  test('renders a multi-line comment as comment tokens with wrapLines (as QueryInput uses)', () => {
    const { container } = render(
      <PrismLight language={CYPHER_LANGUAGE} wrapLines wrapLongLines>
        {'/*\n multi line\n*/\nMATCH (n) RETURN n'}
      </PrismLight>
    );
    const comments = Array.from(container.querySelectorAll('.token.comment'));
    expect(comments.length).toBeGreaterThan(0);
    expect(comments.map((c) => c.textContent).join('')).toContain('multi line');
  });

  test('still renders single-line comments and keywords', () => {
    const { container } = render(
      <PrismLight language={CYPHER_LANGUAGE}>{'MATCH (n) // trailing'}</PrismLight>
    );
    expect(container.querySelector('.token.comment')?.textContent).toContain('// trailing');
    expect(container.querySelector('.token.keyword')?.textContent).toBe('MATCH');
  });
});
