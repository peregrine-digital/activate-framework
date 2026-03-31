/**
 * Tests for the navigation state manager.
 *
 * The navigation module manages page history stack with
 * navigateTo, navigateBack, and reset operations.
 */
import { describe, it, expect } from 'vitest';
import { createNavigation } from '../navigation.svelte';

describe('createNavigation', () => {
  it('starts on the main page', () => {
    const nav = createNavigation();
    expect(nav.page).toBe('main');
  });

  it('navigates to a new page', () => {
    const nav = createNavigation();
    nav.navigateTo('settings');
    expect(nav.page).toBe('settings');
  });

  it('navigates back to previous page', () => {
    const nav = createNavigation();
    nav.navigateTo('settings');
    nav.navigateTo('usage');
    nav.navigateBack();
    expect(nav.page).toBe('settings');
  });

  it('navigates back to main from first level', () => {
    const nav = createNavigation();
    nav.navigateTo('settings');
    nav.navigateBack();
    expect(nav.page).toBe('main');
  });

  it('stays on main when navigating back from main', () => {
    const nav = createNavigation();
    nav.navigateBack();
    expect(nav.page).toBe('main');
  });

  it('reset returns to main and clears history', () => {
    const nav = createNavigation();
    nav.navigateTo('settings');
    nav.navigateTo('usage');
    nav.reset();
    expect(nav.page).toBe('main');
    nav.navigateBack();
    expect(nav.page).toBe('main');
  });

  it('supports multiple navigation levels', () => {
    const nav = createNavigation();
    nav.navigateTo('settings');
    nav.navigateTo('workspace-settings');
    nav.navigateTo('usage');
    expect(nav.page).toBe('usage');
    nav.navigateBack();
    expect(nav.page).toBe('workspace-settings');
    nav.navigateBack();
    expect(nav.page).toBe('settings');
    nav.navigateBack();
    expect(nav.page).toBe('main');
  });
});
