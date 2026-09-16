import { ArrowUpRight } from "lucide-react";
import { Link } from "react-router-dom";
import type { ButtonHTMLAttributes, ReactNode } from "react";

export function ActionLink({
  to,
  children,
  className = "",
}: {
  to: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <Link className={`action-link ${className}`} to={to}>
      {children}
      <span className="action-arrow">
        <ArrowUpRight size={22} />
      </span>
    </Link>
  );
}

export function ActionButton({
  children,
  className = "",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button className={`action-link ${className}`} {...props}>
      {children}
      <span className="action-arrow">
        <ArrowUpRight size={22} />
      </span>
    </button>
  );
}

export function Button({
  children,
  className = "",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button className={`button ${className}`} {...props}>
      {children}
    </button>
  );
}
