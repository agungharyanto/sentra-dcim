import { useId } from "react";

import { useUserText } from "./text";

/** Role picker: one checkbox per role with its description. `locked` roles are shown but cannot be changed. */
export function RoleCheckboxes({
  roles,
  value,
  onChange,
  disabled = false,
  locked = [],
  invalid = false,
  describedBy,
}: {
  roles: readonly string[];
  value: string[];
  onChange: (next: string[]) => void;
  disabled?: boolean;
  locked?: string[];
  invalid?: boolean;
  describedBy?: string;
}) {
  const id = useId();
  const { roleLabel, roleDescription } = useUserText();
  const toggle = (key: string, on: boolean) =>
    onChange(on ? [...value, key] : value.filter((r) => r !== key));
  return (
    <ul className="grid gap-2 sm:grid-cols-2" aria-describedby={describedBy} data-testid="role-checkboxes">
      {roles.map((key) => {
        const cid = `${id}-${key}`;
        const isLocked = locked.includes(key);
        return (
          <li key={key}>
            <label
              htmlFor={cid}
              className="flex h-full cursor-pointer items-start gap-3 rounded-md border p-3 has-[:checked]:border-primary has-[:checked]:bg-accent has-[:disabled]:cursor-not-allowed has-[:disabled]:opacity-70"
            >
              <input
                id={cid}
                type="checkbox"
                className="mt-0.5 size-4 shrink-0 accent-primary"
                checked={value.includes(key)}
                disabled={disabled || isLocked}
                aria-invalid={invalid ? true : undefined}
                onChange={(e) => toggle(key, e.target.checked)}
                data-role={key}
              />
              <span className="min-w-0">
                <span className="block text-sm font-medium">{roleLabel(key)}</span>
                <span className="block text-xs text-muted-foreground">{roleDescription(key)}</span>
              </span>
            </label>
          </li>
        );
      })}
    </ul>
  );
}
