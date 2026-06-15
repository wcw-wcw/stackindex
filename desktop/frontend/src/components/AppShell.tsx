import { ReactNode } from 'react';

export function AppShell({ children }: { children: ReactNode }) {
  return <main className="shell">{children}</main>;
}
