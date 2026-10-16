import { formatCountdown, secondsLeft } from './countdown';

describe('countdown', () => {
  it('counts down to an end time', () => {
    const end = '2026-10-17T10:12:00Z';
    expect(secondsLeft(end, Date.parse('2026-10-17T10:00:00Z'))).toBe(720);
    expect(secondsLeft(end, Date.parse('2026-10-17T10:11:59.200Z'))).toBe(1);
    expect(secondsLeft(end, Date.parse('2026-10-17T10:12:30Z'))).toBe(-30);
  });

  it('formats and stops at zero', () => {
    expect(formatCountdown(720)).toBe('12:00');
    expect(formatCountdown(65)).toBe('1:05');
    expect(formatCountdown(-30)).toBe('0:00');
  });
});
