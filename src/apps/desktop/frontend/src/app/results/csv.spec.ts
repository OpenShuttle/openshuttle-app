import { csvField, toCsv } from './csv';

describe('csv', () => {
  it('quotes only when needed', () => {
    expect(csvField('Zola', ';')).toBe('Zola');
    expect(csvField('BC Lyon; Vaise', ';')).toBe('"BC Lyon; Vaise"');
    expect(csvField('Le "Volant"', ';')).toBe('"Le ""Volant"""');
    expect(csvField('a,b', ';')).toBe('a,b');
    expect(csvField('a,b', ',')).toBe('"a,b"');
  });

  it('builds lines with CRLF', () => {
    expect(toCsv([['Rank', 'Name'], [1, 'Zola']], ';')).toBe('Rank;Name\r\n1;Zola\r\n');
  });
});
