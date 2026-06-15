import { SectionId, sections } from './sections';

export function Sidebar({ active, onSelect }: { active: SectionId; onSelect: (section: SectionId) => void }) {
  return (
    <aside className="sidebar" aria-label="Report sections">
      {sections.map((section) => (
        <button
          key={section.id}
          type="button"
          className={active === section.id ? 'active' : ''}
          onClick={() => onSelect(section.id)}
        >
          {section.label}
        </button>
      ))}
    </aside>
  );
}
