import { backendError } from './backend-error';

describe('backendError', () => {
  it('reads the code and set from the cause', () => {
    expect(backendError({ cause: { code: 'INVALID_SET_SCORE', set: 2 } })).toEqual({
      code: 'INVALID_SET_SCORE',
      set: 2,
    });
  });

  it('reads a code without set', () => {
    expect(backendError({ cause: { code: 'FILE_EXISTS' } })).toEqual({ code: 'FILE_EXISTS' });
  });

  it('falls back to INTERNAL', () => {
    expect(backendError(new Error('boom'))).toEqual({ code: 'INTERNAL' });
    expect(backendError(null)).toEqual({ code: 'INTERNAL' });
    expect(backendError({ cause: { code: 42 } })).toEqual({ code: 'INTERNAL' });
  });
});
