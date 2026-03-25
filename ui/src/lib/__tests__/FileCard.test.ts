import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import FileCard from '../components/FileCard.svelte';
import type { FileStatus } from '../types.js';

function makeFile(overrides: Partial<FileStatus> = {}): FileStatus {
  return {
    dest: 'instructions/general.md',
    displayName: 'General',
    description: 'General coding conventions',
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
  };
}

function makeProps(overrides: Record<string, unknown> = {}) {
  return {
    file: makeFile(),
    installed: true,
    onInstall: vi.fn(),
    onUninstall: vi.fn(),
    onDiff: vi.fn(),
    onSkipUpdate: vi.fn(),
    onOpen: vi.fn(),
    onSetOverride: vi.fn(),
    ...overrides,
  };
}

describe('FileCard', () => {
  describe('row click → onOpen', () => {
    it('calls onOpen when an installed row is clicked', async () => {
      const props = makeProps({ installed: true, file: makeFile({ installed: true, installedVersion: '1.0.0' }) });
      const { container } = render(FileCard, { props });

      const row = container.querySelector('.file-row')!;
      await fireEvent.click(row);

      expect(props.onOpen).toHaveBeenCalledOnce();
      expect(props.onOpen).toHaveBeenCalledWith(props.file);
    });

    it('does NOT call onOpen when a non-installed row is clicked', async () => {
      const props = makeProps({ installed: false });
      const { container } = render(FileCard, { props });

      const row = container.querySelector('.file-row')!;
      await fireEvent.click(row);

      expect(props.onOpen).not.toHaveBeenCalled();
    });

    it('does NOT call onOpen when an action button is clicked', async () => {
      const props = makeProps({
        installed: true,
        file: makeFile({ installed: true, installedVersion: '1.0.0' }),
      });
      const { container } = render(FileCard, { props });

      const actionBtn = container.querySelector('.file-actions button')!;
      expect(actionBtn).toBeTruthy();
      await fireEvent.click(actionBtn);

      expect(props.onOpen).not.toHaveBeenCalled();
    });

    it('calls onOpen when clicking the file-info area', async () => {
      const props = makeProps({ installed: true, file: makeFile({ installed: true, installedVersion: '1.0.0' }) });
      const { container } = render(FileCard, { props });

      const fileInfo = container.querySelector('.file-info')!;
      await fireEvent.click(fileInfo);

      expect(props.onOpen).toHaveBeenCalledOnce();
    });

    it('calls onOpen when clicking the file name text', async () => {
      const props = makeProps({ installed: true, file: makeFile({ installed: true, installedVersion: '1.0.0' }) });
      const { container } = render(FileCard, { props });

      const fileName = container.querySelector('.file-name')!;
      await fireEvent.click(fileName);

      expect(props.onOpen).toHaveBeenCalledOnce();
    });
  });

  describe('action buttons', () => {
    it('uninstall button calls onUninstall (not onOpen)', async () => {
      const props = makeProps({
        installed: true,
        file: makeFile({ installed: true, installedVersion: '1.0.0' }),
      });
      const { container } = render(FileCard, { props });

      const uninstallBtn = container.querySelector('button[title="Uninstall"]')!;
      expect(uninstallBtn).toBeTruthy();
      await fireEvent.click(uninstallBtn);

      expect(props.onUninstall).toHaveBeenCalledOnce();
      expect(props.onOpen).not.toHaveBeenCalled();
    });

    it('install button calls onInstall (not onOpen)', async () => {
      const props = makeProps({ installed: false, file: makeFile() });
      const { container } = render(FileCard, { props });

      const installBtn = container.querySelector('button[title="Install"]')!;
      expect(installBtn).toBeTruthy();
      await fireEvent.click(installBtn);

      expect(props.onInstall).toHaveBeenCalledOnce();
      expect(props.onOpen).not.toHaveBeenCalled();
    });

    it('pin button calls onSetOverride with "pinned"', async () => {
      const props = makeProps({
        installed: true,
        file: makeFile({ installed: true, installedVersion: '1.0.0', override: '' }),
      });
      const { container } = render(FileCard, { props });

      const pinBtn = container.querySelector('button[title*="Pin"]')!;
      expect(pinBtn).toBeTruthy();
      await fireEvent.click(pinBtn);

      expect(props.onSetOverride).toHaveBeenCalledWith(props.file.dest, 'pinned');
      expect(props.onOpen).not.toHaveBeenCalled();
    });

    it('exclude button calls onSetOverride with "excluded"', async () => {
      const props = makeProps({ installed: false, file: makeFile({ override: '' }) });
      const { container } = render(FileCard, { props });

      const excludeBtn = container.querySelector('button[title*="Exclude"]')!;
      expect(excludeBtn).toBeTruthy();
      await fireEvent.click(excludeBtn);

      expect(props.onSetOverride).toHaveBeenCalledWith(props.file.dest, 'excluded');
      expect(props.onOpen).not.toHaveBeenCalled();
    });

    it('diff + skip + update buttons work for outdated files', async () => {
      const props = makeProps({
        installed: true,
        file: makeFile({
          installed: true,
          installedVersion: '0.9.0',
          bundledVersion: '1.0.0',
          updateAvailable: true,
        }),
      });
      const { container } = render(FileCard, { props });

      const diffBtn = container.querySelector('button[title="Show diff"]')!;
      const skipBtn = container.querySelector('button[title="Skip update"]')!;
      const updateBtn = container.querySelector('button[title="Update"]')!;

      expect(diffBtn).toBeTruthy();
      expect(skipBtn).toBeTruthy();
      expect(updateBtn).toBeTruthy();

      await fireEvent.click(diffBtn);
      expect(props.onDiff).toHaveBeenCalledOnce();
      expect(props.onOpen).not.toHaveBeenCalled();

      await fireEvent.click(skipBtn);
      expect(props.onSkipUpdate).toHaveBeenCalledOnce();
      expect(props.onOpen).not.toHaveBeenCalled();

      await fireEvent.click(updateBtn);
      expect(props.onInstall).toHaveBeenCalledOnce();
      expect(props.onOpen).not.toHaveBeenCalled();
    });
  });

  describe('rendering', () => {
    it('shows ✓ status icon when installed and up-to-date', () => {
      const { container } = render(FileCard, {
        props: makeProps({ installed: true, file: makeFile({ installed: true, installedVersion: '1.0.0' }) }),
      });
      const status = container.querySelector('.file-status')!;
      expect(status.textContent).toContain('✓');
    });

    it('shows ○ status icon when not installed', () => {
      const { container } = render(FileCard, {
        props: makeProps({ installed: false }),
      });
      const status = container.querySelector('.file-status')!;
      expect(status.textContent).toContain('○');
    });

    it('shows ⬆ status icon when outdated', () => {
      const { container } = render(FileCard, {
        props: makeProps({
          installed: true,
          file: makeFile({ installed: true, installedVersion: '0.9.0', bundledVersion: '1.0.0' }),
        }),
      });
      const status = container.querySelector('.file-status')!;
      expect(status.textContent).toContain('⬆');
    });

    it('displays file name from displayName', () => {
      const { container } = render(FileCard, {
        props: makeProps({ file: makeFile({ displayName: 'My Custom Name' }) }),
      });
      const name = container.querySelector('.file-name')!;
      expect(name.textContent).toContain('My Custom Name');
    });

    it('displays file description when present', () => {
      const { container } = render(FileCard, {
        props: makeProps({ file: makeFile({ description: 'A helpful description' }) }),
      });
      const desc = container.querySelector('.file-desc')!;
      expect(desc.textContent).toContain('A helpful description');
    });

    it('shows pinned badge when override is "pinned"', () => {
      const { container } = render(FileCard, {
        props: makeProps({
          installed: true,
          file: makeFile({ installed: true, installedVersion: '1.0.0', override: 'pinned' }),
        }),
      });
      const badge = container.querySelector('.file-badge[title*="Pinned"]');
      expect(badge).toBeTruthy();
    });

    it('shows excluded badge when override is "excluded"', () => {
      const { container } = render(FileCard, {
        props: makeProps({
          installed: false,
          file: makeFile({ override: 'excluded' }),
        }),
      });
      const badge = container.querySelector('.file-badge[title*="Excluded"]');
      expect(badge).toBeTruthy();
    });

    it('has cursor-pointer class when installed', () => {
      const { container } = render(FileCard, {
        props: makeProps({ installed: true, file: makeFile({ installed: true, installedVersion: '1.0.0' }) }),
      });
      const row = container.querySelector('.file-row')!;
      expect(row.classList.contains('cursor-pointer')).toBe(true);
    });

    it('does NOT have cursor-pointer class when not installed', () => {
      const { container } = render(FileCard, {
        props: makeProps({ installed: false }),
      });
      const row = container.querySelector('.file-row')!;
      expect(row.classList.contains('cursor-pointer')).toBe(false);
    });
  });
});
