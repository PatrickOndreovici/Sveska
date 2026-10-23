import { useEffect, useRef, useState, type ChangeEvent, type SubmitEvent } from "react";
import { Avatar } from "../../components/Avatar";
import { Button } from "../../components/ui/Button";
import { Eyebrow } from "../../components/ui/Eyebrow";
import { TextField } from "../../components/ui/TextField";
import { ApiError, errorMessage } from "../../lib/api/client";
import { AVATAR_TYPES, MAX_AVATAR_BYTES, deleteAvatar, updateProfile, uploadAvatar } from "../../lib/api/profile";
import type { User } from "../../lib/api/types";
import { roleLabels } from "../auth/navigation";
import { useAuth } from "../auth/useAuth";

/** The logged-in user's profile: picture and display name. Open to both roles. */
export function SettingsPage() {
  const { user, updateUser } = useAuth();
  if (!user) return null;

  return (
    <div className="max-w-prose">
      <header>
        <Eyebrow>Settings</Eyebrow>
        <h1 className="mt-3 font-serif text-4xl text-ink sm:text-display">Podešavanja</h1>
        <p className="mt-4 font-serif text-lg text-ink-muted">How you appear in Sveska.</p>
      </header>

      <div className="mt-10 grid gap-10">
        <PictureSection user={user} onSaved={updateUser} />
        <NameSection user={user} onSaved={updateUser} />
        <section aria-labelledby="account-heading" className="border-t border-rule pt-8">
          <h2 id="account-heading" className="font-serif text-xl text-ink">
            Account
          </h2>
          <dl className="mt-4 grid gap-3 text-sm sm:grid-cols-[8rem_1fr]">
            <dt className="text-ink-muted">Email</dt>
            <dd className="text-ink">{user.email}</dd>
            <dt className="text-ink-muted">Account type</dt>
            <dd className="text-ink">{roleLabels[user.role]}</dd>
          </dl>
          <p className="mt-4 text-sm text-ink-subtle">Your email and account type cannot be changed.</p>
        </section>
      </div>
    </div>
  );
}

type SectionProps = {
  user: User;
  onSaved: (user: User) => void;
};

/** A file picker that uploads the chosen picture straight away, and a way to remove it. */
function PictureSection({ user, onSaved }: SectionProps) {
  const [busy, setBusy] = useState<"uploading" | "removing" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);
  // Focus goes back to the picker once it is enabled again, which is only
  // after the render that clears busy.
  const [focusRequests, setFocusRequests] = useState(0);

  useEffect(() => {
    if (focusRequests > 0) inputRef.current?.focus();
  }, [focusRequests]);

  async function handleChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    // Cleared so choosing the same file again still fires a change.
    event.target.value = "";
    if (!file) return;
    setError(null);
    setStatus("");
    if (file.size > MAX_AVATAR_BYTES) {
      setError("Choose a picture of at most 10 MB.");
      return;
    }

    setBusy("uploading");
    try {
      onSaved(await uploadAvatar(file));
      setStatus("Your picture is saved.");
    } catch (err) {
      setError(
        err instanceof ApiError && err.fields["file"]
          ? err.fields["file"]
          : errorMessage(err, "Could not save the picture. Try again."),
      );
    } finally {
      setBusy(null);
      setFocusRequests((n) => n + 1);
    }
  }

  async function handleRemove() {
    setError(null);
    setStatus("");
    setBusy("removing");
    try {
      await deleteAvatar();
      onSaved({ ...user, avatarUrl: null });
      setStatus("Your picture is removed.");
      // The remove button is gone now; keep focus in the section.
      setFocusRequests((n) => n + 1);
    } catch (err) {
      setError(errorMessage(err, "Could not remove the picture. Try again."));
    } finally {
      setBusy(null);
    }
  }

  return (
    <section aria-labelledby="picture-heading">
      <h2 id="picture-heading" className="font-serif text-xl text-ink">
        Profile picture
      </h2>
      <div className="mt-4 flex flex-col gap-6 sm:flex-row sm:items-center">
        <Avatar user={user} size="lg" alt={user.avatarUrl ? "Your profile picture" : "Your initials"} />
        <div className="grid min-w-0 flex-1 gap-1.5">
          <label htmlFor="avatar-upload" className="text-sm font-medium text-ink">
            {user.avatarUrl ? "Choose a new picture" : "Choose a picture"}
          </label>
          <input
            ref={inputRef}
            id="avatar-upload"
            type="file"
            accept={AVATAR_TYPES}
            disabled={busy !== null}
            aria-describedby="avatar-upload-hint"
            aria-invalid={error ? true : undefined}
            onChange={(event) => {
              void handleChange(event);
            }}
            className="block w-full text-sm text-ink-muted file:mr-3 file:h-8 file:cursor-pointer file:rounded-sm file:border file:border-rule-strong file:bg-paper-raised file:px-3 file:text-sm file:font-medium file:text-ink hover:file:border-ink disabled:opacity-50"
          />
          <p id="avatar-upload-hint" className="text-sm text-ink-subtle">
            JPEG, PNG, GIF or WebP up to 10 MB. It is cut to a square from the middle.
          </p>
          {user.avatarUrl && (
            <div>
              <Button
                variant="quiet"
                size="sm"
                className="h-auto px-0 py-1"
                disabled={busy !== null}
                onClick={() => {
                  void handleRemove();
                }}
              >
                Remove picture
              </Button>
            </div>
          )}
          {error && (
            <p role="alert" className="text-sm text-danger">
              {error}
            </p>
          )}
          {/* Always rendered so screen readers announce the text when it appears. */}
          <p role="status" className="text-sm text-ink-muted empty:hidden">
            {busy === "uploading" ? "Saving your picture…" : busy === "removing" ? "Removing your picture…" : status}
          </p>
        </div>
      </div>
    </section>
  );
}

function NameSection({ user, onSaved }: SectionProps) {
  const [error, setError] = useState<string | undefined>(undefined);
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  async function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const value = new FormData(event.currentTarget).get("displayName");
    const displayName = typeof value === "string" ? value.trim() : "";
    setError(undefined);
    setFormError(null);
    setSaved(false);
    if (!displayName) {
      setError("Enter your name.");
      return;
    }

    setSaving(true);
    try {
      onSaved(await updateProfile({ displayName }));
      setSaved(true);
    } catch (err) {
      if (err instanceof ApiError && err.fields["displayName"]) setError(err.fields["displayName"]);
      else setFormError(errorMessage(err, "Could not save your name. Try again."));
    } finally {
      setSaving(false);
    }
  }

  return (
    <section aria-labelledby="name-heading" className="border-t border-rule pt-8">
      <h2 id="name-heading" className="font-serif text-xl text-ink">
        Name
      </h2>
      <form
        noValidate
        onSubmit={(event) => {
          void handleSubmit(event);
        }}
        className="mt-4 grid gap-4"
      >
        <TextField
          id="display-name"
          name="displayName"
          label="Display name"
          hint="Shown in the header and on your workspace."
          autoComplete="name"
          maxLength={80}
          defaultValue={user.displayName}
          error={error}
          onChange={() => {
            setSaved(false);
          }}
        />
        {formError && (
          <p role="alert" className="text-sm text-danger">
            {formError}
          </p>
        )}
        <div className="flex flex-wrap items-center gap-4">
          <Button type="submit" disabled={saving}>
            Save name
          </Button>
          <p role="status" className="text-sm text-ink-muted empty:hidden">
            {saving ? "Saving…" : saved ? "Saved." : ""}
          </p>
        </div>
      </form>
    </section>
  );
}
