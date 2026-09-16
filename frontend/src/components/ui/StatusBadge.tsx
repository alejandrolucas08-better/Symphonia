import type { ReactNode } from "react";

export function StatusBadge({
  children,
  active = true,
}: {
  children: ReactNode;
  active?: boolean;
}) {
  return (
    <span className={`status ${active ? "is-active" : ""}`}>
      <span className="status-dot" />
      {children}
    </span>
  );
}
