import React from 'react';
import { render } from '@testing-library/react';
import { describe, test, expect } from 'vitest';
import HistoryModal from '../HistoryModal';

describe('HistoryModal Component', () => {
  // Regression: the history view must use the same extended Cypher grammar as
  // the editor so multi-line (/* */) comments are highlighted consistently.
  // Because the highlighter is rendered with an inline-style theme (dracula),
  // token types are conveyed via inline color rather than a `.comment` class —
  // so we assert the comment body shares the color of the `/*` opener. The
  // stock `cypher` grammar would not tokenize `/* */` as a comment at all, so
  // the interior text would not be a single comment-colored token.
  test('highlights multi-line comments in history entries', () => {
    const { container } = render(
      <HistoryModal
        isOpen={true}
        onClose={() => {}}
        history={['/*\n multi line\n*/\nMATCH (n) RETURN n']}
        onSelectQuery={() => {}}
      />
    );

    const leafSpans = Array.from(container.querySelectorAll('span')).filter(
      (s) => s.children.length === 0
    ) as HTMLElement[];

    const opener = leafSpans.find((s) => s.textContent?.includes('/*'));
    expect(opener, 'expected a `/*` comment token').toBeTruthy();
    const commentColor = opener!.style.color;
    expect(commentColor).not.toBe('');

    const body = leafSpans.find((s) => s.textContent?.includes('multi line'));
    expect(body, 'expected the comment body to be a single token').toBeTruthy();
    expect(body!.style.color).toBe(commentColor);
  });

  test('renders nothing when closed', () => {
    const { container } = render(
      <HistoryModal
        isOpen={false}
        onClose={() => {}}
        history={['MATCH (n) RETURN n']}
        onSelectQuery={() => {}}
      />
    );
    expect(container.firstChild).toBeNull();
  });
});
