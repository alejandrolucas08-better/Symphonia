import { useId } from "react";
import { languages } from "../../data/mocks";
import type { LanguageCode } from "../../types/demo";

export function LanguageSelect({
  label,
  value,
  onChange,
}: {
  label: string;
  value: LanguageCode;
  onChange: (value: LanguageCode) => void;
}) {
  const id = useId();
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <select
        id={id}
        value={value}
        onChange={(event) => onChange(event.target.value as LanguageCode)}
      >
        {languages.map((language) => (
          <option key={language.code} value={language.code}>
            {language.name}
          </option>
        ))}
      </select>
    </div>
  );
}
