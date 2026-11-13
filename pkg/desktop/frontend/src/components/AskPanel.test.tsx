import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { PendingAsk } from '../state/chat';
import { AskPanel } from './AskPanel';

const { respondAsk } = vi.hoisted(() => ({ respondAsk: vi.fn() }));

vi.mock('../state/actions', () => ({ respondAsk }));

function makeAsk(questions: PendingAsk['params']['questions'], agentName?: string): PendingAsk {
  return { requestId: 'req-1', params: { sessionId: 'session', agentName, questions }, answers: {} };
}

describe('AskPanel', () => {
  beforeEach(() => {
    respondAsk.mockReset();
    respondAsk.mockResolvedValue(undefined);
  });

  afterEach(() => {
    cleanup();
  });

  it('shows one question at a time and switches with tabs', () => {
    render(
      <AskPanel
        ask={makeAsk([
          { id: 'q1', question: 'First question?', type: 'text' },
          { id: 'q2', question: 'Second question?', type: 'text' },
        ])}
      />,
    );
    expect(screen.getByText('First question?', { selector: '.ask-question-text' })).toBeTruthy();
    expect(screen.queryByText('Second question?', { selector: '.ask-question-text' })).toBeNull();
    fireEvent.click(screen.getByRole('tab', { name: /Second/ }));
    expect(screen.getByText('Second question?', { selector: '.ask-question-text' })).toBeTruthy();
    expect(screen.queryByText('First question?', { selector: '.ask-question-text' })).toBeNull();
  });

  it('submits a text answer with Enter', () => {
    render(<AskPanel ask={makeAsk([{ id: 'q1', question: 'Your name?', type: 'text', required: true }])} />);
    const input = screen.getByPlaceholderText('Type your answer…');
    fireEvent.change(input, { target: { value: 'Pegg' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(respondAsk).toHaveBeenCalledWith('req-1', { q1: 'Pegg' });
  });

  it('blocks submit when a required answer is missing', () => {
    render(<AskPanel ask={makeAsk([{ id: 'q1', question: 'Your name?', type: 'text', required: true }])} />);
    fireEvent.click(screen.getByRole('button', { name: 'Submit' }));
    expect(respondAsk).not.toHaveBeenCalled();
    expect(screen.getByText('This answer is required')).toBeTruthy();
  });

  it('submits a custom answer for a select question', () => {
    render(
      <AskPanel ask={makeAsk([{ id: 'q1', question: 'Pick one', type: 'select', options: ['A', 'B'] }])} />,
    );
    fireEvent.click(screen.getByRole('button', { name: 'Custom…' }));
    const input = screen.getByPlaceholderText('Type a custom answer…');
    fireEvent.change(input, { target: { value: 'Z' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(respondAsk).toHaveBeenCalledWith('req-1', { q1: 'Z' });
  });

  it('appends a custom answer to a multiselect question', () => {
    render(
      <AskPanel
        ask={makeAsk([{ id: 'q1', question: 'Pick many', type: 'multiselect', options: ['A', 'B'] }])}
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: 'A' }));
    fireEvent.click(screen.getByRole('button', { name: 'Custom…' }));
    const input = screen.getByPlaceholderText('Type a custom answer…');
    fireEvent.change(input, { target: { value: 'Z' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    fireEvent.click(screen.getByRole('button', { name: 'Submit' }));
    expect(respondAsk).toHaveBeenCalledWith('req-1', { q1: 'A, Z' });
  });

  it('rejects an empty custom answer', () => {
    render(
      <AskPanel ask={makeAsk([{ id: 'q1', question: 'Pick one', type: 'select', options: ['A'] }])} />,
    );
    fireEvent.click(screen.getByRole('button', { name: 'Custom…' }));
    fireEvent.keyDown(screen.getByPlaceholderText('Type a custom answer…'), { key: 'Enter' });
    expect(screen.getByText('Enter a custom answer')).toBeTruthy();
    expect(respondAsk).not.toHaveBeenCalled();
  });

  it('walks multiple questions and submits from the review step', () => {
    render(
      <AskPanel
        ask={makeAsk([
          { id: 'q1', question: 'First?', type: 'text' },
          { id: 'q2', question: 'Second?', type: 'text' },
        ])}
      />,
    );
    let input = screen.getByPlaceholderText('Type your answer…');
    fireEvent.change(input, { target: { value: 'one' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    input = screen.getByPlaceholderText('Type your answer…');
    fireEvent.change(input, { target: { value: 'two' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(screen.getByText('Review your answers')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Submit' }));
    expect(respondAsk).toHaveBeenCalledWith('req-1', { q1: 'one', q2: 'two' });
  });

  it('cancels with an empty answer map', () => {
    render(<AskPanel ask={makeAsk([{ id: 'q1', question: 'Q?', type: 'text' }])} />);
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(respondAsk).toHaveBeenCalledWith('req-1', {});
  });
});
