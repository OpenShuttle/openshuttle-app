import { guideUrl } from './guide';

describe('guide', () => {
  it('points to the guide in the UI language', () => {
    expect(guideUrl('fr')).toBe('https://openshuttle.app/guide/');
    expect(guideUrl('en')).toBe('https://openshuttle.app/en/guide/');
  });
});
