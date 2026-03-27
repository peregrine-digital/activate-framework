/**
 * Tests for MainPage file categorization logic.
 *
 * MainPage categorizes files into: installed (in-tier), available,
 * outside-tier, and excluded. These tests verify the filtering logic
 * matches the expected behavior when tier, install state, and overrides change.
 */
import { describe, it, expect } from 'vitest';
import type { FileStatus } from '../types';

// Extract the categorization logic from MainPage for unit testing
function categorize(files: FileStatus[]) {
  const installed = files.filter((f) => f.installed && f.inTier && f.override !== 'excluded');
  const available = files.filter((f) => !f.installed && f.inTier && f.override !== 'excluded');
  const outsideTier = files.filter((f) => !f.inTier && f.override !== 'excluded');
  const excluded = files.filter((f) => f.override === 'excluded');
  return { installed, available, outsideTier, excluded };
}

const makeFile = (overrides: Partial<FileStatus>): FileStatus => ({
  dest: 'test.md',
  displayName: 'test',
  category: 'instructions',
  tier: 'core',
  installed: false,
  inTier: true,
  bundledVersion: '1.0.0',
  installedVersion: null,
  updateAvailable: false,
  skipped: false,
  override: '',
  ...overrides,
});

describe('File categorization', () => {
  it('installed + inTier → installed category', () => {
    const files = [makeFile({ installed: true, inTier: true })];
    const { installed, available, outsideTier } = categorize(files);
    expect(installed).toHaveLength(1);
    expect(available).toHaveLength(0);
    expect(outsideTier).toHaveLength(0);
  });

  it('installed + NOT inTier → outsideTier category', () => {
    const files = [makeFile({ installed: true, inTier: false })];
    const { installed, outsideTier } = categorize(files);
    expect(installed).toHaveLength(0);
    expect(outsideTier).toHaveLength(1);
  });

  it('not installed + inTier → available category', () => {
    const files = [makeFile({ installed: false, inTier: true })];
    const { installed, available } = categorize(files);
    expect(installed).toHaveLength(0);
    expect(available).toHaveLength(1);
  });

  it('not installed + NOT inTier → outsideTier category', () => {
    const files = [makeFile({ installed: false, inTier: false })];
    const { outsideTier } = categorize(files);
    expect(outsideTier).toHaveLength(1);
  });

  it('excluded override → excluded category regardless of other flags', () => {
    const files = [
      makeFile({ installed: true, inTier: true, override: 'excluded' }),
      makeFile({ installed: false, inTier: true, override: 'excluded' }),
      makeFile({ installed: false, inTier: false, override: 'excluded' }),
    ];
    const { installed, available, outsideTier, excluded } = categorize(files);
    expect(installed).toHaveLength(0);
    expect(available).toHaveLength(0);
    expect(outsideTier).toHaveLength(0);
    expect(excluded).toHaveLength(3);
  });

  it('pinned override does NOT affect categorization', () => {
    const files = [makeFile({ installed: true, inTier: true, override: 'pinned' })];
    const { installed } = categorize(files);
    expect(installed).toHaveLength(1);
  });

  it('tier change moves installed files to outsideTier', () => {
    // Simulate switching from "workflow" tier (all in tier) to "skills" tier
    const files = [
      makeFile({ dest: 'skill1.md', installed: true, inTier: true, tier: 'skills' }),
      makeFile({ dest: 'skill2.md', installed: true, inTier: true, tier: 'skills' }),
      makeFile({ dest: 'agent1.md', installed: true, inTier: false, tier: 'workflow' }),
      makeFile({ dest: 'agent2.md', installed: true, inTier: false, tier: 'workflow' }),
    ];
    const { installed, outsideTier } = categorize(files);
    expect(installed).toHaveLength(2);
    expect(outsideTier).toHaveLength(2);
    expect(outsideTier.map((f) => f.dest)).toEqual(['agent1.md', 'agent2.md']);
  });

  it('mixed state files sort correctly', () => {
    const files = [
      makeFile({ dest: 'a.md', installed: true, inTier: true }),
      makeFile({ dest: 'b.md', installed: false, inTier: true }),
      makeFile({ dest: 'c.md', installed: false, inTier: false }),
      makeFile({ dest: 'd.md', installed: true, inTier: false }),
      makeFile({ dest: 'e.md', override: 'excluded' }),
    ];
    const { installed, available, outsideTier, excluded } = categorize(files);
    expect(installed).toHaveLength(1);
    expect(available).toHaveLength(1);
    expect(outsideTier).toHaveLength(2);
    expect(excluded).toHaveLength(1);
  });
});
