# UI Component Library - Design System

This directory contains the core design system components for the Subcults application. All components follow consistent styling patterns using Tailwind CSS with no inline styles.

## Components

### Modal

Dialog/modal component with focus management and accessibility.

**Features:**

- Focus trap
- ESC key to close
- Backdrop click to close (configurable)
- Accessible (role="dialog", aria-modal)
- Multiple sizes (sm, md, lg, xl)
- Animated entrance

**Usage:**

```tsx
import { Modal, ConfirmModal } from '@/components/ui';

// Basic modal
<Modal
  isOpen={isOpen}
  onClose={handleClose}
  title="Edit Profile"
  footer={
    <>
      <Button variant="ghost" onClick={handleClose}>Cancel</Button>
      <Button variant="primary" onClick={handleSave}>Save</Button>
    </>
  }
>
  <p>Modal content goes here</p>
</Modal>

// Confirm modal preset
<ConfirmModal
  isOpen={isOpen}
  onClose={handleClose}
  onConfirm={handleDelete}
  title="Delete Account"
  message="Are you sure? This action cannot be undone."
  confirmText="Delete"
  cancelText="Cancel"
  variant="danger"
  isLoading={isDeleting}
/>
```

---

### LoadingSpinner

Reusable loading indicator for async operations.

**Sizes:**

- `sm` - 4x4 (16px)
- `md` - 6x6 (24px)
- `lg` - 8x8 (32px)
- `xl` - 12x12 (48px)

**Features:**

- Accessible with aria-label
- Screen reader support
- Consistent animation

**Usage:**

```tsx
import { LoadingSpinner, FullPageLoader } from '@/components/ui';

// Inline spinner
<LoadingSpinner size="md" label="Loading data" />

// Full page loader
<FullPageLoader label="Loading application" />

// In a button (handled by Button component)
<Button isLoading>Saving...</Button>
```

---

## Design Tokens

### Colors

**Brand Colors:**

- Primary: `#7C3AED` (Neon purple)
- Primary Dark: `#5B21B6`
- Accent: `#00FFFF` (Cyan)
- Secondary Accent: `#FF00FF` (Magenta)
- Success Accent: `#00FF41` (Neon green)

**Semantic Colors:**

- Success: `green-500`
- Error: `red-500`, `red-600`, `red-700`
- Info: `blue-500`

**Surface Colors:**

- Background: CSS var `--color-background`
- Background Secondary: CSS var `--color-background-secondary`
- Background Hover: CSS var `--color-background-hover`
- Border: CSS var `--color-border`
- Border Hover: CSS var `--color-border-hover`

### Typography

**Font Family:**

```text
'Space Mono', monospace
```

**Font Sizes:**

- `text-xs` - 0.75rem (12px)
- `text-sm` - 0.875rem (14px)
- `text-base` - 1rem (16px)
- `text-lg` - 1.125rem (18px)
- `text-xl` - 1.25rem (20px)
- `text-2xl` - 1.5rem (24px)

### Spacing

**Touch Targets:**

- Minimum touch target: `44px` (accessible)
- Smaller acceptable: `36px` (for dense UIs)

**Gaps:**

- `gap-2` - 0.5rem (8px)
- `gap-3` - 0.75rem (12px)
- `gap-4` - 1rem (16px)

### Border Radius

- `rounded-none` - 0px (default for most components)
- `rounded-full` - 50% (allowed for badges, avatars, spinners)

### Animations

**Durations:**

- Fast: `200ms` - fades
- Standard: `250ms` - colors, opacity
- Moderate: `300ms` - slides, transforms

**Available Animations:**

- `animate-fade-in` - Fade in entrance
- `animate-slide-up` - Slide up from bottom
- `animate-slide-in` - Slide in from right (toasts and panels)
- `animate-spin` - Continuous rotation (spinners)

---

## Accessibility Guidelines

### Focus Indicators

All interactive components use visible focus indicators:

```css
focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-primary
```

### Color Contrast

All color combinations meet WCAG AA standards:

- Text on background: ≥4.5:1
- Large text on background: ≥3:1
- UI components: ≥3:1

### ARIA Attributes

- Buttons: `aria-label` for icon-only buttons
- Inputs: `aria-describedby` for errors/helpers
- Modals: `role="dialog"`, `aria-modal="true"`, `aria-labelledby`
- Loading states: `role="status"`, `aria-live="polite"`

### Keyboard Navigation

- Tab navigation works on all interactive elements
- Focus trap in modals
- ESC to close modals
- Enter to submit forms

---

## Testing

All components have comprehensive test coverage:

```bash
# Run all UI component tests
npm test -- ui/ --run

# Run specific component tests
npm test -- Modal.test.tsx --run
npm test -- LoadingSpinner.test.tsx --run
```

---

## Migration Guide

Pages use CSS utility classes (`button-primary`, `button-secondary`, `button-quiet`, `button-danger`, `field`) defined in `index.css` instead of dedicated button or input components.

---

## Future Enhancements

Planned components:

- [ ] Checkbox component
- [ ] Radio button component
- [ ] Select/Dropdown component
- [ ] Textarea component
- [ ] Tooltip component
- [ ] Badge component
- [ ] Alert/Banner component
- [ ] Card component
- [ ] Tabs component
- [ ] Accordion component

---

## Contributing

When adding new components:

1. Create component in `/components/ui/`
2. Follow existing patterns (Tailwind only, no inline styles)
3. Add comprehensive tests
4. Include accessibility features
5. Document usage in this README
6. Export from `index.ts`
7. Ensure WCAG AA compliance
