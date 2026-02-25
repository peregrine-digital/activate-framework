const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const { selectFiles, DEFAULT_TIERS, listByCategory, inferCategory, parseManifestData } = require('../manifest');

/**
 * Inline copy of parseFrontmatterVersion for testing without vscode dependency.
 * Must stay in sync with installer.js implementation.
 */
function parseFrontmatterVersion(buffer) {
  const text = Buffer.from(buffer).toString('utf8');
  const match = text.match(/^---\s*\n([\s\S]*?)\n---/);
  if (!match) return null;
  const fm = match[1];
  const versionLine = fm.match(/^version:\s*['"]?([^'"\n]+)['"]?\s*$/m);
  return versionLine ? versionLine[1].trim() : null;
}

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

describe('DEFAULT_TIERS', () => {
  it('contains the three expected tiers', () => {
    const tierIds = DEFAULT_TIERS.map((t) => t.id).sort();
    assert.deepEqual(tierIds, ['advanced', 'minimal', 'standard']);
  });

  it('each tier includes all previous tier file tags (cumulative)', () => {
    const minimal = new Set(DEFAULT_TIERS.find((t) => t.id === 'minimal').includes);
    const standard = new Set(DEFAULT_TIERS.find((t) => t.id === 'standard').includes);
    const advanced = new Set(DEFAULT_TIERS.find((t) => t.id === 'advanced').includes);

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

  it('infers mcp-servers from path', () => {
    assert.equal(inferCategory('mcp-servers/github.json'), 'mcp-servers');
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

describe('parseManifestData', () => {
  it('parses a full manifest data object', () => {
    const data = {
      name: 'Test Manifest',
      description: 'A test manifest',
      version: '1.2.3',
      files: [{ src: 'foo.md', dest: 'foo.md', tier: 'core' }],
    };
    const result = parseManifestData('test-id', data);
    assert.equal(result.id, 'test-id');
    assert.equal(result.name, 'Test Manifest');
    assert.equal(result.description, 'A test manifest');
    assert.equal(result.version, '1.2.3');
    assert.equal(result.files.length, 1);
  });

  it('uses id as name when name is missing', () => {
    const result = parseManifestData('my-manifest', { files: [] });
    assert.equal(result.name, 'my-manifest');
    assert.equal(result.description, '');
    assert.equal(result.version, 'unknown');
    assert.deepEqual(result.files, []);
  });

  it('defaults files to empty array when missing', () => {
    const result = parseManifestData('no-files', {});
    assert.deepEqual(result.files, []);
  });
});

describe('parseFrontmatterVersion', () => {
  it('extracts version from standard frontmatter', () => {
    const buf = Buffer.from("---\ndescription: 'test'\nversion: '0.5.0'\n---\n# Hello");
    assert.equal(parseFrontmatterVersion(buf), '0.5.0');
  });

  it('extracts version without quotes', () => {
    const buf = Buffer.from('---\nversion: 1.2.3\n---\n# Hello');
    assert.equal(parseFrontmatterVersion(buf), '1.2.3');
  });

  it('returns null when no frontmatter', () => {
    const buf = Buffer.from('# Just a heading\nSome content');
    assert.equal(parseFrontmatterVersion(buf), null);
  });

  it('returns null when no version in frontmatter', () => {
    const buf = Buffer.from("---\ndescription: 'test'\n---\n# Hello");
    assert.equal(parseFrontmatterVersion(buf), null);
  });
});
