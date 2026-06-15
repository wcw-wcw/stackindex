export const sections = [
  { id: 'overview', label: 'Overview' },
  { id: 'audit', label: 'Audit' },
  { id: 'context', label: 'Context' },
  { id: 'routes', label: 'API Routes' },
  { id: 'tests', label: 'Tests' },
  { id: 'ask', label: 'Ask' },
  { id: 'ai', label: 'AI Notes' },
  { id: 'reports', label: 'Reports' },
] as const;

export type SectionId = typeof sections[number]['id'];
