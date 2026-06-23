# StackIndex Desktop Design

## Goal

StackIndex desktop should feel like the current terminal TUI translated into a standalone app. It is an operational report workspace first: dense, local-first, readable, and quiet.

## Visual Rules

- Use flat charcoal backgrounds with slightly varied dark panels.
- Keep monospace-first typography.
- Use simple borders instead of shadows, glow, or glossy surfaces.
- Use cyan for StackIndex/title accents.
- Use muted gray for metadata and inactive navigation.
- Use green for success, amber for warnings, red for high severity, and purple for the active TUI-style selection row.
- Keep spacing compact. Report screens should feel dense but readable.
- Preserve readable code/path styling with wrapping for long local paths.

## Avoid

- Blue/black gradients or decorative background washes.
- Oversized hero sections and marketing-page spacing.
- Glossy cards, floating SaaS dashboards, and heavy shadows.
- Bright white or light-mode surfaces.
- Excessive rounded pills or badge overload.
- Visual patterns copied from StackIndex-adjacent projects.

## Component Rules

- Panels: flat charcoal fill, one-pixel border, small radius, compact padding.
- Sidebar: terminal navigation feel, muted inactive rows, purple active row, visible `>` marker.
- Buttons: tool-like controls with flat fills and borders; primary actions may use purple, not glossy blue.
- Chips: stack labels should read like terminal-separated text rather than modern product pills.
- Paths/code: monospace, dark code background, wrapping enabled for long local paths.
- Statuses: concise terminal badges or colored text, using the severity colors above.

Future polish should preserve this identity. Improve clarity and accessibility without drifting toward a generic SaaS dashboard, portfolio site, or stock-app visual language.
