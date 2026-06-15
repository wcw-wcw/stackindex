export function MetricCard({ label, value, tone }: { label: string; value: number | string; tone?: 'danger' | 'warning' | 'success' }) {
  return (
    <div className={`metric-card ${tone ? `tone-${tone}` : ''}`}>
      <strong>{value}</strong>
      <span>{label}</span>
    </div>
  );
}
