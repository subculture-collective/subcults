# Subcults Design System

**Style**: Bold Typography (Editorial Dark)
**Last Updated**: 2026-02-28

---

## Design Philosophy

**Confident. Editorial. Deliberate.** Subcults is a manifesto for underground music communities, not a polished SaaS product. The design treats typography as the primary visual structure - text is the hero, space is the frame, and interaction reveals detail. Every word earns its place.

### Core Principles

1. **Type as Hero**: Headlines are the visual centerpiece. A well-set 80pt headline is more compelling than any stock photo. Scene names, event titles, and discovery headers drive the visual hierarchy.

2. **Extreme Scale Contrast**: 6:1+ ratio between H1 and body text. This creates editorial drama that separates Subcults from generic SaaS platforms.

3. **Deliberate Negative Space**: Dark space is the frame around type. Generous margins make content feel intentional, curated - not algorithmic.

4. **Strict Hierarchy**: Every element has a clear rank. The eye flows: headline > subhead > body > action. No two elements compete for attention.

5. **Restrained Palette**: Near-black, warm white, and one accent (vermillion). More colors dilute typographic impact. Let the letter shapes do the work.

### The Vibe

The page feels like a gallery exhibition or underground music zine. Every word is placed with intent. Visual signatures:
- Massive headlines that command scroll
- Tight letter-spacing on display text (`-0.04em` to `-0.06em`)
- Wide letter-spacing on labels (`0.1em` to `0.2em`)
- Text that bleeds to edge on mobile
- Underlines as the primary interactive affordance
- Sharp corners throughout - no rounded edges except avatars and pills

### What This Design Is NOT

- Not friendly SaaS with rounded cards and soft shadows
- Not generic dark mode with colors inverted
- Not minimalist to the point of being boring
- Not a copy of Spotify/SoundCloud - this is editorial, not streaming UI

---

## Design Token System

### Colors (Dark Mode Primary)

Subcults is dark-first. The dark palette is the definitive identity.

```
background:        #0A0A0A    // Near-black canvas (never pure #000)
background-alt:    #0F0F0F    // Slight elevation from canvas
foreground:        #FAFAFA    // Warm white primary text
muted:             #1A1A1A    // Elevated surfaces, cards, panels
muted-foreground:  #737373    // Secondary text, descriptions, metadata
accent:            #FF3D00    // Vermillion - warm, urgent, visible
accent-foreground: #0A0A0A    // Dark text on accent surfaces
accent-hover:      #FF5722    // Lighter vermillion for hover states
border:            #262626    // Barely-there structural dividers
border-hover:      #404040    // Borders on interaction
input:             #1A1A1A    // Input field backgrounds
card:              #0F0F0F    // Card elevation from canvas
ring:              #FF3D00    // Focus ring color (matches accent)
```

**Light Mode Adaptation** (secondary):
```
background:        #FAFAFA
background-alt:    #F5F5F5
foreground:        #0A0A0A
muted:             #E5E5E5
muted-foreground:  #737373
accent:            #FF3D00    // Vermillion stays consistent
accent-foreground: #FAFAFA
border:            #E5E5E5
border-hover:      #FF3D00
```

**Color Usage Rules**:
- Vermillion is for interactive elements, live indicators, and key CTAs only - never decoration
- Secondary text uses `muted-foreground` (#737373) exclusively - no random grays
- Full-width horizontal rules use `border` color
- "Live" streaming indicators can pulse in vermillion
- Scene/event status badges use vermillion sparingly

**Contrast Ratios**:
- `foreground` on `background`: 18.1:1 (exceeds AAA)
- `muted-foreground` on `background`: 5.3:1 (meets AA)
- `accent` on `background`: 5.4:1 (meets AA for large text)
- `accent` on `muted`: 5.1:1 (meets AA for large text)

### Typography

**Font Stack**:
- **Headlines**: `"Inter Tight", "Inter", system-ui, sans-serif` - Tighter default spacing, clean geometric forms
- **Body**: `"Inter", system-ui, sans-serif` - Clean, highly readable at all sizes
- **Mono/Data**: `"JetBrains Mono", "Fira Code", monospace` - Technical precision for stats, coordinates, timestamps

```css
@import url('https://fonts.googleapis.com/css2?family=Inter+Tight:wght@400;500;600;700;800;900&family=Inter:wght@400;500;600&family=JetBrains+Mono:wght@400;500&display=swap');
```

**Tailwind Config**:
```js
fontFamily: {
  display: ['"Inter Tight"', '"Inter"', 'system-ui', 'sans-serif'],
  sans: ['"Inter"', 'system-ui', 'sans-serif'],
  mono: ['"JetBrains Mono"', '"Fira Code"', 'monospace'],
}
```

**Type Scale**:
```
xs:    0.75rem    (12px)  — fine print, timestamps
sm:    0.875rem   (14px)  — captions, metadata
base:  1rem       (16px)  — body text
lg:    1.125rem   (18px)  — lead paragraphs, scene descriptions
xl:    1.25rem    (20px)  — subheads
2xl:   1.5rem     (24px)  — section intros
3xl:   2rem       (32px)  — H3 (scene names in lists)
4xl:   2.5rem     (40px)  — H2 (section headers)
5xl:   3.5rem     (56px)  — H1 mobile
6xl:   4.5rem     (72px)  — H1 tablet
7xl:   6rem       (96px)  — H1 desktop
8xl:   8rem       (128px) — Hero statement
9xl:   10rem      (160px) — Decorative numbers (scene counts, stats)
```

**Tracking**:
```
tighter:  -0.06em   — Display headlines (hero, discovery header)
tight:    -0.04em   — Large headings (section titles)
normal:   -0.01em   — Body text (slightly tightened from browser default)
wide:     0.05em    — Small labels
wider:    0.1em     — All-caps labels, navigation
widest:   0.2em     — Sparse emphasis (section tags)
```

**Line Heights**:
```
none:     1         — Single-line headlines
tight:    1.1       — Multi-line headlines
snug:     1.25      — Subheads
normal:   1.6       — Body text
relaxed:  1.75      — Long-form reading (event descriptions)
```

**Type Treatment**:
- Headlines use `font-display` (Inter Tight), weight 600-900, `tracking-tighter`
- Body uses `font-sans` (Inter), weight 400-500
- Data labels (coordinates, timestamps, counts) use `font-mono`, weight 400, `tracking-wider`, `uppercase`
- All-caps used for section labels, navigation, badges, and status indicators
- Scene names and event titles stay in title case for readability

### Border Radius

```
radius:       0px     — Default. Sharp corners everywhere.
Exception:    rounded-full — ONLY for avatars, pill badges, and status dots
```

No `rounded-md`, `rounded-lg`, `rounded-xl`. It's either sharp or fully round. This creates the editorial, gallery-like precision.

### Shadows & Effects

**No traditional shadows.** Depth comes from:
- Typography scale contrast (large muted text behind smaller bright text)
- Accent underlines (2-3px vermillion lines under interactive elements)
- Full-width horizontal rules (dividers)
- Background color alternation (`background` and `muted`)

```
shadow:       none
textShadow:   none
```

**Subtle Noise Grain**: A barely-visible fractal noise pattern at 1.5% opacity overlays the page background, adding tactile quality to the dark canvas without distraction.

```css
.bg-noise::after {
  content: '';
  position: fixed;
  inset: 0;
  pointer-events: none;
  opacity: 0.015;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 256 256' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noise'%3E%3CfeTurbulence baseFrequency='0.65' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noise)'/%3E%3C/svg%3E");
}
```

**Accent Bars**: Thin horizontal vermillion bars (`h-1`, `w-16`) serve as visual anchors on key section headers and featured content.

---

## Component Styles

### Buttons

**Primary Button** (text-only with animated underline):
```
- No background fill
- Text: accent color (#FF3D00)
- Animated underline: absolute span, h-0.5, bg-accent
- Base: scale-x-100 → hover: scale-x-110
- Uppercase, tracking-wider (0.1em)
- Font-weight: 600 (semibold)
- Padding: py-2 (sm), py-3 (md), py-4 (lg) — px-0
- Active: translate-y-px (subtle press)
- Transition: 150ms all
- Focus-visible: ring-2 ring-accent ring-offset-2
```

**Secondary/Outline Button**:
```
- Border: 1px solid foreground
- Text: foreground
- No background initially
- Hover: bg-foreground, text becomes background (full inversion)
- Sharp corners (0px radius)
- Padding: px-6 py-3
- Uppercase, tracking-wider
```

**Ghost Button**:
```
- No border, no fill
- Text: muted-foreground
- Hover: text becomes foreground
- Underline appears via scale-x-0 → scale-x-100
- h-px underline (thinner than primary)
```

**All Buttons**:
- `min-h-[44px]` touch target
- `focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2`
- `disabled:pointer-events-none disabled:opacity-50`
- `whitespace-nowrap`

### Cards / Containers

**Minimal card usage.** Content separates primarily by:
- Generous section padding (`py-20` to `py-40`)
- Full-width horizontal borders (`border-t` / `border-b`)
- Typography scale changes
- Background color alternation (`background` ↔ `muted`)

When cards are necessary (scene cards, event cards, pricing):
```
- Border: 1px solid border color
- Background: transparent
- No radius (sharp corners)
- No shadow
- Padding: p-6 (mobile) → p-8 (desktop)
- Hover: border color shifts to border-hover (150ms)
```

**Featured/Highlighted Cards** (promoted scenes, live events):
```
- Border: 2px solid accent
- Small accent badge above content (bg-accent, px-3 py-1, uppercase mono)
- No background change — border is the differentiator
```

**Map Overlay Cards** (scene detail panels):
```
- Background: background with 95% opacity for map readability
- Border: 1px solid border
- Backdrop-filter: blur(4px) — subtle for legibility without heavy glass effect
- Sharp corners
```

### Inputs

```
- Background: input color (#1A1A1A)
- Border: 1px solid border
- Border-radius: 0px (sharp)
- Height: h-12 (mobile) → h-14 (desktop)
- Font-size: text-base (16px — prevents iOS zoom)
- Padding: px-4
- Text: foreground
- Placeholder: muted-foreground
- Focus: border-accent, outline-none, no ring, no glow
- Transition: colors 150ms
- Disabled: cursor-not-allowed, opacity-50
```

### Navigation

```
- Background: background (solid, not transparent)
- Border-bottom: 1px solid border
- Links: font-mono, uppercase, tracking-wider, text-sm
- Active link: text-accent with accent underline
- Hover: text-foreground (from muted-foreground)
- Logo: font-display, font-bold, tracking-tight
```

### Badges / Tags

```
- Scene genre tags: border 1px solid border, text-xs, font-mono, uppercase, tracking-widest
- Live indicator: rounded-full, bg-accent, h-2 w-2, animate-pulse
- Status badges: px-3 py-1, font-mono, text-xs, uppercase
```

---

## Layout Strategy

### Container
```
maxWidth:  1200px (max-w-5xl)
padding:   px-6 (mobile), px-12 (tablet), px-16 (desktop)
```

### Section Spacing
```
py-20   (80px)  — tight sections
py-28   (112px) — standard sections
py-40   (160px) — hero / major CTA sections
```

### Grid Philosophy
- **Asymmetric grids**: 7/5 or 8/4 column splits instead of 6/6
- **Staggered alignment**: Elements don't always align top
- **Text columns**: max-w-2xl for readability, headlines can span full width
- **Map layout**: Map takes primary space, side panels use narrow columns

### Section Dividers
- Full-width `border-t border-border`
- Gradient line accents: `bg-gradient-to-r from-transparent via-white/10 to-transparent`
- Background color alternation between sections

---

## Effects & Animation

### Motion Philosophy

**Fast and decisive.** No bouncy easing. No playful delays. Movement is confident and direct — like the underground music scenes Subcults serves.

```
duration: 150ms  — micro-interactions (buttons, underlines, focus)
duration: 200ms  — standard transitions (card borders, color changes)
duration: 500ms  — image hover effects, panel transitions
easing:   cubic-bezier(0.25, 0, 0, 1)  — fast-out, crisp stop
```

### Specific Effects

**Link/Button interactions**:
- Underline scale animation (`scale-x-0` → `scale-x-100` on hover)
- Text color transition (150ms)
- Active press feedback: `translate-y-px`
- No scale, no glow, no bounce

**Card hover**:
- Border color lightens to `border-hover`
- No lift, no shadow, no scale
- Background color change on feature cards (transparent → muted)

**Page scroll animations** (optional, Framer Motion):
- Fade in + slide up (opacity 0→1, translateY 20px→0) over 500ms
- Stagger children by 80ms
- Viewport trigger: once only, 15% threshold
- **Must respect** `prefers-reduced-motion`

**Live streaming indicators**:
- Vermillion dot with `animate-pulse` (loading indicators only)
- Audio waveform visualization - no infinite decorative animations

### Animation Accessibility
- All animations wrapped in `@media (prefers-reduced-motion: no-preference)`
- Reduced motion fallback: instant state changes, no transitions
- No infinite decorative animations (only functional: loading spinners, live indicators)
- Use `ease-out` for entering elements, `ease-in` for exiting (never `linear` for UI transitions)

---

## Iconography

From `lucide-react`:
```
- Stroke width: 1.5px (thinner than default for editorial elegance)
- Size by context:
  - 16px: inline with small text, button arrows
  - 18px: navigation, controls
  - 20px: standard icons
  - 24px: feature section, empty states
- Color: currentColor (inherits text color)
- Accent icons: explicitly text-accent
- Style: Always outline/stroke — never filled
- Use sparingly — text labels preferred over icon-only buttons
```

---

## Responsive Strategy

### Mobile-First Typography Scaling
- Hero headline: `text-4xl` → `text-5xl` (sm) → `text-6xl` (md) → `text-7xl` (lg) → `text-8xl` (xl)
- Section titles: `text-3xl` → `text-4xl` (md) → `text-5xl` (lg)
- Body text: `text-base` throughout, `md:text-lg` on key sections
- Maintain hierarchy ratio at all breakpoints

### Layout Shifts
- Scene grid: 1 col → 2 col (sm) → 3 col (lg)
- Event list: stacked → 2 col (md)
- Map + panel: stacked (mobile) → side-by-side (lg)
- Footer: 2 col → 4 col (md)
- Asymmetric grids collapse to stacked on mobile

### Spacing Adjustments
- Section padding: `py-20` (mobile) → `py-28` (md) → `py-32`/`py-40` (lg)
- Container: `px-6` (mobile) → `px-12` (md) → `px-16` (lg)
- Gap: `gap-4` → `gap-6` → `gap-8` progression

### Mobile-Specific
- Touch targets minimum 44x44px (`min-h-touch min-w-touch`)
- Reserve space for dynamic content with skeleton loaders (avoid layout shifts)
- Keep editorial aesthetic on mobile — don't strip personality for smaller screens
- Stack email/search inputs with buttons on mobile, side-by-side on tablet+
- Navigation collapses to hamburger with full-screen overlay

### Breakpoints
```
xs:   375px   — iPhone SE
sm:   640px   — Small tablets
md:   768px   — Tablets / landscape phones
lg:   1024px  — Desktop
xl:   1280px  — Large desktop
2xl:  1536px  — Ultra-wide
```

---

## Accessibility Requirements

### WCAG 2.1 AA Compliance (Minimum)

**Contrast**:
- Primary text: 18.1:1 ratio (exceeds AAA)
- Secondary text: 5.3:1 ratio (meets AA)
- Accent on dark: 5.4:1 ratio (meets AA for large text)
- All text-color combinations must pass AA minimum

**Focus States**:
- 2px accent outline on all interactive elements
- 2px offset from element edge
- No glow, no fill change — outline only
- Visible on all interactive elements including custom components

**Typography**:
- Body text minimum 16px (prevents iOS zoom)
- Line-height minimum 1.5 for body
- No thin weights below 400

**Interaction**:
- Touch targets minimum 44x44px
- Underlines 2px+ for visibility
- Color is never the sole indicator — always pair with icon, label, or position
- Error messages use `aria-live="polite"` or `role="alert"`
- All images require descriptive `alt` text

**Motion**:
- `prefers-reduced-motion` respected for all animations
- Essential interactions work without animation
- No auto-playing audio or video

---

## Tailwind Configuration Reference

The following shows how to adapt `tailwind.config.js` for this design system:

```js
// tailwind.config.js
export default {
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        background: {
          DEFAULT: 'var(--color-background)',
          alt: 'var(--color-background-alt)',
        },
        foreground: {
          DEFAULT: 'var(--color-foreground)',
          muted: 'var(--color-foreground-muted)',
        },
        accent: {
          DEFAULT: 'var(--color-accent)',
          foreground: 'var(--color-accent-foreground)',
          hover: 'var(--color-accent-hover)',
        },
        border: {
          DEFAULT: 'var(--color-border)',
          hover: 'var(--color-border-hover)',
        },
        muted: {
          DEFAULT: 'var(--color-muted)',
          foreground: 'var(--color-muted-foreground)',
        },
        input: 'var(--color-input)',
        card: 'var(--color-card)',
        ring: 'var(--color-ring)',
      },
      fontFamily: {
        display: ['"Inter Tight"', '"Inter"', 'system-ui', 'sans-serif'],
        sans: ['"Inter"', 'system-ui', 'sans-serif'],
        mono: ['"JetBrains Mono"', '"Fira Code"', 'monospace'],
      },
      borderRadius: {
        none: '0px',  // Default for all elements
      },
      letterSpacing: {
        tighter: '-0.06em',
        tight: '-0.04em',
        normal: '-0.01em',
        wide: '0.05em',
        wider: '0.1em',
        widest: '0.2em',
      },
    },
  },
}
```

### CSS Variables

```css
:root {
  /* Dark mode (default/primary) */
  --color-background: #0A0A0A;
  --color-background-alt: #0F0F0F;
  --color-foreground: #FAFAFA;
  --color-foreground-muted: #737373;
  --color-accent: #FF3D00;
  --color-accent-foreground: #0A0A0A;
  --color-accent-hover: #FF5722;
  --color-border: #262626;
  --color-border-hover: #404040;
  --color-muted: #1A1A1A;
  --color-muted-foreground: #737373;
  --color-input: #1A1A1A;
  --color-card: #0F0F0F;
  --color-ring: #FF3D00;
}

.light {
  --color-background: #FAFAFA;
  --color-background-alt: #F5F5F5;
  --color-foreground: #0A0A0A;
  --color-foreground-muted: #737373;
  --color-accent: #FF3D00;
  --color-accent-foreground: #FAFAFA;
  --color-accent-hover: #FF5722;
  --color-border: #E5E5E5;
  --color-border-hover: #FF3D00;
  --color-muted: #E5E5E5;
  --color-muted-foreground: #737373;
  --color-input: #F5F5F5;
  --color-card: #FFFFFF;
  --color-ring: #FF3D00;
}
```
