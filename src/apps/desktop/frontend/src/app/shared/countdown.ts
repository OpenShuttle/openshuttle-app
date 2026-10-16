/** Seconds left until an ISO time; negative once elapsed. */
export function secondsLeft(endsAt: string, now: number): number {
  return Math.ceil((Date.parse(endsAt) - now) / 1000);
}

/** "m:ss" for a positive count of seconds, "0:00" once elapsed. */
export function formatCountdown(seconds: number): string {
  const s = Math.max(0, seconds);
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}
