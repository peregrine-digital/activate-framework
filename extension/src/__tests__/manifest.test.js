const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const { selectFiles, TIER_MAP, listByCategory, inferCategory } = require('../manifest');

const mockFiles = [
  { src: 'AGENTS.md', dest: 'AGENTS.md', tier: 'core', category: 'other', description: 'Agent guidelines' },
  { src: 'instructions/general.instructions.md', dest: 'instructions/general.instructions.md', tier: 'core', category: 'instructions', description: 'Universal conventions' },
  { src: 'instructions/python.instructions.md', dest: 'instructions/python.instructions.md', tier: 'ad-hoc', category: 'instructions', description: 'Python conventions' },
  { src: 'skills/advanced-tool/SKILL.md', dest: 'skills/advanced-tool/SKILL.md', tier: 'ad-hoc-advanced', category: 'skills', description: 'Advanced tooling' },
  { src: 'prompts/code-review.prompt.md', dest: 'prompts/code-review.prompt.md', tier: 'core', category: 'prompts', description: 'Code review prompt' },
  { src: 'agents/service-designer.agent.md', dest: 'agents/service-designer.agent.md', tier: 'ad-hoc', category: 'agents', description: 'Service designer' },
];

describe('selectFiles', () => {
  it('minimal tier returns only core files', () => {
    const result = selectFiles(mockFiles, 'minimal');
    assert.equal(result.length, 3);
    assert.ok(result.every((f) => f.tier === 'core'));
  });

  it('standard tier returns core + ad-hoc files', () => {
    const result = selectFiles(mockFiles, 'standard');
    assert.equal(result.length, 5);
    assert.ok(result.every((f) => f.tier === 'core' || f.tier === 'ad-hoc'));
  });

  it('advanced tier returns all files', () => {
    const result = selectFiles(mockFiles, 'advanced');
    assert.equal(result.length, 6);
  });

  it('unknown tier falls back to standard', () => {
    const result = selectFiles(mockFiles, 'bogus');
    assert.equal(result.length, 5);
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

describe('inferCategory', () => {
  it('infers instructions from path', () => {
    assert.equal(inferCategory('instructions/foo.md'), 'instructions');
  });

  it('infers prompts from path', () => {
    assert.equal(inferCategory('prompts/bar.md'), 'prompts');
  });

  it('infers skills from path', () => {
    assert.equal(inferCategory('skills/baz/SKILL.md'), 'skills');
  });

  it('infers agents from path', () => {
    assert.equal(inferCategory('agents/agent.md'), 'agents');
  });

  it('returns other for unknown paths', () => {
    assert.equal(inferCategory('AGENTS.md'), 'other');
  });
});

describe('listByCategory', () => {
  it('groups all files by category in display order', () => {
    const groups = listByCategory(mockFiles);
    const labels = groups.map((g) => g.category);
    assert.deepEqual(labels, ['instructions', 'prompts', 'skills', 'agents', 'other']);
  });

  it('filters by tier when specified', () => {
    const groups = listByCategory(mockFiles, { tier: 'minimal' });
    const allFiles = groups.flatMap((g) => g.files);
    assert.ok(allFiles.every((f) => f.tier === 'core'));
    assert.equal(allFiles.length, 3);
  });

  it('filters by category when specified', () => {
    const groups = listByCategory(mockFiles, { category: 'instructions' });
    assert.equal(groups.length, 1);
    assert.equal(groups[0].category, 'instructions');
    assert.equal(groups[0].files.length, 2);
  });

  it('combines tier and category filters', () => {
    const groups = listByCategory(mockFiles, { tier: 'minimal', category: 'instructions' });
    assert.equal(groups.length, 1);
    assert.equal(groups[0].files.length, 1);
    assert.equal(groups[0].files[0].dest, 'instructions/general.instructions.md');
  });

  it('returns empty when no files match', () => {
    const groups = listByCategory(mockFiles, { tier: 'minimal', category: 'skills' });
    assert.equal(groups.length, 0);
  });
});
