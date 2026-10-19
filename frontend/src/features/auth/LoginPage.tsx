import type { SubmitEvent } from "react";
import { useLocation, useNavigate } from "react-router";
import { Button } from "../../components/ui/Button";
import { FormAlert } from "../../components/ui/FormAlert";
import { TextField } from "../../components/ui/TextField";
import { TextLink } from "../../components/ui/TextLink";
import { AuthPageLayout } from "./AuthPageLayout";
import type { RedirectState } from "./routeGuards";
import { textValue, useAuthForm } from "./useAuthForm";
import { useAuth } from "./useAuth";

export function LoginPage() {
  const { logIn } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const { formRef, submitting, formError, fieldErrors, submit } = useAuthForm();

  const redirectTo = (location.state as RedirectState | null)?.from ?? "/";

  function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    void submit(async () => {
      await logIn({ email: textValue(data, "email"), password: textValue(data, "password") });
      await navigate(redirectTo, { replace: true });
    });
  }

  return (
    <AuthPageLayout title="Log in" intro="Pick up where you left off.">
      <form ref={formRef} noValidate onSubmit={handleSubmit} className="grid gap-5">
        {formError && <FormAlert>{formError}</FormAlert>}
        <TextField
          id="login-email"
          name="email"
          type="email"
          label="Email"
          autoComplete="email"
          spellCheck={false}
          error={fieldErrors["email"]}
        />
        <TextField
          id="login-password"
          name="password"
          type="password"
          label="Password"
          autoComplete="current-password"
          error={fieldErrors["password"]}
        />
        <Button type="submit" disabled={submitting} className="mt-1 w-full">
          {submitting ? "Logging in…" : "Log in"}
        </Button>
      </form>
      <p className="text-sm text-ink-muted">
        New to Sveska? <TextLink to="/signup">Create an account</TextLink>
      </p>
    </AuthPageLayout>
  );
}
