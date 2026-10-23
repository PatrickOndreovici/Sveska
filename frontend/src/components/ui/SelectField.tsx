import { Select, type SelectProps } from "./Select";

export type SelectFieldProps = Omit<SelectProps, "id" | "className" | "size"> & {
  id: string;
  label: string;
};

/** Labelled select, styled to match TextField. */
export function SelectField({ id, label, ...selectProps }: SelectFieldProps) {
  return (
    <div className="grid gap-1.5">
      <label htmlFor={id} className="text-sm font-medium text-ink">
        {label}
      </label>
      <Select id={id} {...selectProps} />
    </div>
  );
}
