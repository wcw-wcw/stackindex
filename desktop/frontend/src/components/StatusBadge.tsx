export function StatusBadge({ status }: { status: string }) {
  const normalized = status.toLowerCase();
  const tone = normalized.includes('pass') || normalized.includes('generated') ? 'success'
    : normalized.includes('fail') || normalized.includes('unavailable') ? 'danger'
      : normalized.includes('warning') || normalized.includes('not') ? 'warning'
        : 'neutral';

  return <span className={`status-badge tone-${tone}`}>{status}</span>;
}
