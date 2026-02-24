const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const { selectFiles, TIER_MAP } = require('../manifest');

const mockFiles = [
  { src: 'AGENTS.md', dest: 'AGENTS.md', tier: 'core' },
  { src: 'instructions/general.instructions.md', dest: 'instructions/general.instructions.md', tier: 'core' },
  { src: 'instructions/python.instructions.md', dest: 'instructions/python.instructions.md', tier: 'ad-hoc' },
  { src: 'skills/advanced-tool/SKILL.md', dest: 'skills/advanced-tool/SKILL.md', tier: 'ad-hoc-advanced' },
];

describe('selectFiles', () => {
  it('minimal tier returns only core files', () => {
    const result = selectFiles(mockFiles, 'minimal');
    assert.equal(result.length, 2);
    assert.ok(result.every((f) => f.tier === 'core'));
  });

  it('standard tier returns core + ad-hoc files', () => {
    const result = selectFiles(mockFiles, 'standard');
    assert.equal(result.length, 3);
    assert.ok(result.every((f) => f.tier === 'core' || f.tier === 'ad-hoc'));
  });

  it('advanced tier returns all files', () => {
    const result = selectFiles(mockFiles, 'advanced');
    assert.equal(result.length, 4);
  });

  it('unknown tier falls back to standard', () => {
    const result = selectFiles(mockFiles, 'bogus');
    assert.equal(result.length, 3);
  });
});

describe('TIER_MAP', () => {
  it('contains the three expected tiers', () => {
    assert.deepEqual(Object.keys(TIER_MAP).sort(), ['advanced', 'minimal', 'standard']);
  });

  it('each tier is a superset of the previous', () => {
    const minimal = TIER_MAP.minimal;
    const standard = TIER_MAP.standard;
    const advanced = TIER_MAP.advanced;

    for (const t of minimal) assert.ok(standard.has(t), `standard should include "${t}"`);
    for (const t of standard) assert.ok(advanced.has(t), `advanced should include "${t}"`);
  });
});
