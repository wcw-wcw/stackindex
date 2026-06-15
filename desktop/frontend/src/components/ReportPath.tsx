export function ReportPath({ label, path }: { label: string; path: string }) {
  return (
    <div className="report-path">
      <span>{label}</span>
      <code>{path}</code>
    </div>
  );
}
