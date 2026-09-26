import { useCallback, useReducer } from 'react'

const LIMIT = 100

interface State<T> {
  past: T[]
  committed: T
  present: T
  future: T[]
}

type Action<T> =
  | { type: 'preview'; value: T }
  | { type: 'commit'; value: T }
  | { type: 'undo' }
  | { type: 'redo' }
  | { type: 'reset'; value: T }

function same<T>(a: T, b: T) {
  return Object.is(a, b) || JSON.stringify(a) === JSON.stringify(b)
}

function reducer<T>(s: State<T>, a: Action<T>): State<T> {
  switch (a.type) {
    case 'preview':
      return { ...s, present: a.value }
    case 'commit':
      if (same(s.committed, a.value)) return { ...s, present: a.value, committed: a.value }
      return {
        past: [...s.past, s.committed].slice(-LIMIT),
        committed: a.value,
        present: a.value,
        future: [],
      }
    case 'undo': {
      const prev = s.past[s.past.length - 1]
      if (prev === undefined) return s
      return {
        past: s.past.slice(0, -1),
        committed: prev,
        present: prev,
        future: [s.committed, ...s.future],
      }
    }
    case 'redo': {
      const next = s.future[0]
      if (next === undefined) return s
      return {
        past: [...s.past, s.committed],
        committed: next,
        present: next,
        future: s.future.slice(1),
      }
    }
    case 'reset':
      return { past: [], committed: a.value, present: a.value, future: [] }
  }
}

/**
 * Undo/redo history. `preview` changes the current value without creating an
 * undo step (slider drag); `commit` records one step since the last commit.
 */
export function useHistory<T>(initial: T) {
  const [state, dispatch] = useReducer(reducer<T>, {
    past: [],
    committed: initial,
    present: initial,
    future: [],
  })
  return {
    present: state.present,
    preview: useCallback((value: T) => dispatch({ type: 'preview', value }), []),
    commit: useCallback((value: T) => dispatch({ type: 'commit', value }), []),
    undo: useCallback(() => dispatch({ type: 'undo' }), []),
    redo: useCallback(() => dispatch({ type: 'redo' }), []),
    reset: useCallback((value: T) => dispatch({ type: 'reset', value }), []),
    canUndo: state.past.length > 0,
    canRedo: state.future.length > 0,
  }
}
