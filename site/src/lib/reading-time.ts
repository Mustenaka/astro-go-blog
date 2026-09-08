import { site } from '../site.config';

export interface ReadingTime {
  minutes: number;
  /** CJK characters plus Latin words. */
  units: number;
}

/**
 * Estimate reading time from raw MDX. CJK characters and Latin words are counted at
 * different speeds (see `site.readingSpeed`). Import lines and JSX tags are ignored.
 */
export function estimateReadingTime(body: string | undefined): ReadingTime {
  const text = (body ?? '')
    .replace(/^import\s.+$/gm, '')
    .replace(/<[^>\n]+>/g, ' ');
  const cjk = (text.match(/\p{Script=Han}/gu) ?? []).length;
  const words = (text.replace(/\p{Script=Han}/gu, ' ').match(/[A-Za-z0-9_]+/g) ?? []).length;
  const minutes = Math.max(1, Math.round(cjk / site.readingSpeed.cjkCharsPerMinute + words / site.readingSpeed.wordsPerMinute));
  return { minutes, units: cjk + words };
}
