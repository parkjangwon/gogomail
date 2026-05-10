import FormField, { FormFieldProps } from "@cloudscape-design/components/form-field";
import { ReactNode } from "react";

export interface ValidatedFormFieldProps extends Omit<FormFieldProps, "children"> {
  children: ReactNode;
  error?: string;
  touched?: boolean;
}

export function ValidatedFormField({
  error,
  touched,
  children,
  ...props
}: ValidatedFormFieldProps) {
  const showError = touched && error;

  return (
    <FormField
      {...props}
      errorText={showError ? error : ""}
    >
      {children}
    </FormField>
  );
}
