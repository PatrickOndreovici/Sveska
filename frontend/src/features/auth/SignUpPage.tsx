import type { SubmitEvent } from "react";
import { useNavigate } from "react-router";
import { Button } from "../../components/ui/Button";
import { FormAlert } from "../../components/ui/FormAlert";
import { TextField } from "../../components/ui/TextField";
import { TextLink } from "../../components/ui/TextLink";
import { AuthPageLayout } from "./AuthPageLayout";
import { isRole, RoleChoice } from "./RoleChoice";
import { textValue, useAuthForm, type FieldErrors } from "./useAuthForm";
import { useAuth } from "./useAuth";

export function SignUpPage() {
  const { signUp } = useAuth();
  const navigate = useNavigate();
  const { formRef, submitting, formError, fieldErrors, submit } = useAuthForm();

  function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const input = {
      displayName: textValue(data, "displayName").trim(),
      email: textValue(data, "email").trim(),
      password: textValue(data, "password"),
    };
    const role = textValue(data, "role");

    // The server enforces every rule. Here we only catch empty fields, so a
    // blank form gets all its messages at once, and because an unset radio
    // group cannot be sent as a Role.
    const missing: FieldErrors = {};
    if (!input.displayName) missing["displayName"] = "Enter your name.";
    if (!input.email) missing["email"] = "Enter your email address.";
    if (!input.password) missing["password"] = "Enter a password.";
    if (!isRole(role)) missing["role"] = "Choose student or teacher.";
    if (!isRole(role) || Object.keys(missing).length > 0) {
      void submit(() => Promise.resolve(), missing);
      return;
    }
    void submit(async () => {
      await signUp({ ...input, role });
      await navigate("/", { replace: true });
    });
  }

  return (
    <AuthPageLayout title="Create your account" intro="Students can start on their own and invite a teacher at any time.">
      <form ref={formRef} noValidate onSubmit={handleSubmit} className="grid gap-5">
        {formError && <FormAlert>{formError}</FormAlert>}
        <TextField
          id="signup-name"
          name="displayName"
          label="Your name"
          autoComplete="name"
          error={fieldErrors["displayName"]}
        />
        <TextField
          id="signup-email"
          name="email"
          type="email"
          label="Email"
          autoComplete="email"
          spellCheck={false}
          error={fieldErrors["email"]}
        />
        <TextField
          id="signup-password"
          name="password"
          type="password"
          label="Password"
          hint="At least 10 characters."
          autoComplete="new-password"
          error={fieldErrors["password"]}
        />
        <RoleChoice error={fieldErrors["role"]} />
        <Button type="submit" disabled={submitting} className="mt-1 w-full">
          {submitting ? "Creating account…" : "Create account"}
        </Button>
      </form>
      <p className="text-sm text-ink-muted">
        Already have an account? <TextLink to="/login">Log in</TextLink>
      </p>
    </AuthPageLayout>
  );
}
